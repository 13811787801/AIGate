package channel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"aigate/internal/store"
)

func deepSeekSearch(ch store.Channel) bool {
	if strings.EqualFold(strings.TrimSpace(ch.Provider), "deepseek") {
		return true
	}
	return strings.Contains(strings.ToLower(ch.BaseURL), "api.deepseek.com")
}

func deepSeekMessagesURL(base string) string {
	u, err := url.Parse(strings.TrimSpace(base))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "https://api.deepseek.com/anthropic/v1/messages"
	}
	u.Path = "/anthropic/v1/messages"
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

func (a *OpenAICompatibleAdapter) chatWithWebSearch(ctx context.Context, ch store.Channel, upstreamModel string, req ChatRequest) (*ChatResponse, error) {
	maxTokens := 4096
	if req.MaxTokens != nil && *req.MaxTokens > 0 {
		maxTokens = *req.MaxTokens
	}
	payload, err := buildWebSearchBody(upstreamModel, maxTokens, req.Messages)
	if err != nil {
		return nil, err
	}
	body, _ := json.Marshal(payload)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, deepSeekMessagesURL(ch.BaseURL), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	if ch.APIKeyEnc != "" {
		httpReq.Header.Set("x-api-key", ch.APIKeyEnc)
	}
	client := &http.Client{Timeout: 180 * time.Second}
	res, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("upstream %d: %s", res.StatusCode, truncate(string(raw), 500))
	}
	out, err := parseWebSearchResponse(raw)
	if err != nil {
		return nil, err
	}
	out.Model = upstreamModel
	out.Raw = raw
	return out, nil
}

func buildWebSearchBody(model string, maxTokens int, msgs []ChatMessage) (map[string]any, error) {
	var system []string
	messages := make([]map[string]any, 0, len(msgs))
	for _, m := range msgs {
		text := strings.TrimSpace(m.Content)
		if text == "" {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(m.Role))
		if role == "system" {
			system = append(system, text)
			continue
		}
		if role != "assistant" {
			role = "user"
		}
		messages = append(messages, map[string]any{
			"role": role,
			"content": []map[string]string{{
				"type": "text",
				"text": text,
			}},
		})
	}
	if len(messages) == 0 {
		return nil, fmt.Errorf("messages required")
	}
	body := map[string]any{
		"model":      model,
		"max_tokens": maxTokens,
		"messages":   messages,
		"tools": []map[string]any{{
			"type":     "web_search_20250305",
			"name":     "web_search",
			"max_uses": 3,
		}},
		"thinking": map[string]string{"type": "disabled"},
	}
	if len(system) > 0 {
		body["system"] = strings.Join(system, "\n")
	}
	return body, nil
}

func parseWebSearchResponse(raw []byte) (*ChatResponse, error) {
	var payload struct {
		ID         string `json:"id"`
		Type       string `json:"type"`
		StopReason string `json:"stop_reason"`
		Error      *struct {
			Message string `json:"message"`
		} `json:"error"`
		Content []struct {
			Type    string          `json:"type"`
			Text    string          `json:"text"`
			Content json.RawMessage `json:"content"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	if payload.Type == "error" || (payload.Error != nil && payload.Error.Message != "") {
		msg := "web search failed"
		if payload.Error != nil && payload.Error.Message != "" {
			msg = payload.Error.Message
		}
		return nil, fmt.Errorf("%s", msg)
	}
	var texts []string
	var sources []string
	seen := map[string]struct{}{}
	for _, block := range payload.Content {
		switch block.Type {
		case "text":
			if t := strings.TrimSpace(block.Text); t != "" {
				texts = append(texts, t)
			}
		case "web_search_tool_result":
			for _, src := range readSearchSources(block.Content) {
				if _, ok := seen[src.url]; ok {
					continue
				}
				seen[src.url] = struct{}{}
				line := src.url
				if src.title != "" {
					line = src.title + " — " + src.url
				}
				sources = append(sources, line)
			}
		}
	}
	answer := strings.TrimSpace(strings.Join(texts, "\n\n"))
	if answer == "" && len(sources) == 0 {
		return nil, fmt.Errorf("联网搜索没有返回内容")
	}
	if len(sources) > 0 {
		if answer != "" {
			answer += "\n\n"
		}
		answer += "来源：\n- " + strings.Join(sources, "\n- ")
	}
	now := time.Now().Unix()
	inTok := payload.Usage.InputTokens
	outTok := payload.Usage.OutputTokens
	return &ChatResponse{
		ID:      payload.ID,
		Object:  "chat.completion",
		Created: now,
		Choices: []struct {
			Index   int `json:"index"`
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		}{{
			Index: 0,
			Message: struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			}{Role: "assistant", Content: answer},
			FinishReason: payload.StopReason,
		}},
		Usage: Usage{
			PromptTokens:     inTok,
			CompletionTokens: outTok,
			TotalTokens:      inTok + outTok,
		},
	}, nil
}

type searchSource struct {
	title string
	url   string
}

func readSearchSources(raw json.RawMessage) []searchSource {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '[' {
		return nil
	}
	var items []struct {
		Type  string `json:"type"`
		URL   string `json:"url"`
		Title string `json:"title"`
	}
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil
	}
	out := make([]searchSource, 0, len(items))
	for _, item := range items {
		if item.Type != "web_search_result" || strings.TrimSpace(item.URL) == "" {
			continue
		}
		out = append(out, searchSource{title: strings.TrimSpace(item.Title), url: strings.TrimSpace(item.URL)})
	}
	return out
}

func streamOneShot(resp *ChatResponse) <-chan StreamChunk {
	out := make(chan StreamChunk, 2)
	go func() {
		defer close(out)
		content := ""
		if len(resp.Choices) > 0 {
			content = resp.Choices[0].Message.Content
		}
		chunk := map[string]any{
			"id": resp.ID, "object": "chat.completion.chunk", "created": resp.Created, "model": resp.Model,
			"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": content}, "finish_reason": "stop"}},
			"usage":   resp.Usage,
		}
		b, _ := json.Marshal(chunk)
		out <- StreamChunk{Data: b}
		out <- StreamChunk{Done: true}
	}()
	return out
}
