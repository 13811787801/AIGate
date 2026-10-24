package channel

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"aigate/internal/store"
)

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature *float64      `json:"temperature,omitempty"`
	TopP        *float64      `json:"top_p,omitempty"`
	MaxTokens   *int          `json:"max_tokens,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
	// WebSearch asks DeepSeek's native search tool. Omitted on the normal chat call.
	WebSearch bool `json:"web_search,omitempty"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ChatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage Usage           `json:"usage"`
	Raw   json.RawMessage `json:"-"`
}

type StreamChunk struct {
	Data []byte
	Done bool
	Err  error
}

type Adapter interface {
	Provider() string
	Chat(ctx context.Context, ch store.Channel, upstreamModel string, req ChatRequest) (*ChatResponse, error)
	ChatStream(ctx context.Context, ch store.Channel, upstreamModel string, req ChatRequest) (<-chan StreamChunk, error)
}

type Registry struct {
	byProvider map[string]Adapter
}

func NewRegistry(adapters ...Adapter) *Registry {
	r := &Registry{byProvider: map[string]Adapter{}}
	for _, a := range adapters {
		r.byProvider[a.Provider()] = a
	}
	return r
}

// AdapterKind maps commercial vendor labels to a concrete adapter.
// Most cloud vendors speak OpenAI-compatible HTTP; only mock is special-cased.
func AdapterKind(provider string) string {
	p := strings.TrimSpace(strings.ToLower(provider))
	switch p {
	case "", "openai_compatible", "custom",
		"openai", "azure", "anthropic", "claude", "gemini", "google",
		"deepseek", "qwen", "dashscope", "moonshot", "kimi", "zhipu", "glm",
		"doubao", "volcengine", "baichuan", "siliconflow", "groq", "mistral",
		"openrouter", "ollama", "minimax", "yi", "lingyi", "hunyuan", "tencent":
		return "openai_compatible"
	case "mock":
		return "mock"
	default:
		// Unknown vendor: still try OpenAI-compatible (common commercial pattern).
		return "openai_compatible"
	}
}

func (r *Registry) Get(provider string) (Adapter, error) {
	kind := AdapterKind(provider)
	a, ok := r.byProvider[kind]
	if !ok {
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
	return a, nil
}

// --- mock ---

type MockAdapter struct{}

func (MockAdapter) Provider() string { return "mock" }

func (MockAdapter) Chat(ctx context.Context, ch store.Channel, upstreamModel string, req ChatRequest) (*ChatResponse, error) {
	_ = ctx
	_ = ch
	prompt := lastUserContent(req.Messages)
	content := "mock reply: " + prompt
	inTok := estimateTokens(prompt)
	outTok := estimateTokens(content)
	now := time.Now().Unix()
	return &ChatResponse{
		ID:      "chatcmpl-mock",
		Object:  "chat.completion",
		Created: now,
		Model:   upstreamModel,
		Choices: []struct {
			Index   int `json:"index"`
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		}{
			{Index: 0, Message: struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			}{Role: "assistant", Content: content}, FinishReason: "stop"},
		},
		Usage: Usage{PromptTokens: inTok, CompletionTokens: outTok, TotalTokens: inTok + outTok},
	}, nil
}

func (m MockAdapter) ChatStream(ctx context.Context, ch store.Channel, upstreamModel string, req ChatRequest) (<-chan StreamChunk, error) {
	out := make(chan StreamChunk, 8)
	go func() {
		defer close(out)
		resp, err := m.Chat(ctx, ch, upstreamModel, req)
		if err != nil {
			out <- StreamChunk{Err: err}
			return
		}
		content := resp.Choices[0].Message.Content
		// 简易 SSE：一个 delta + done
		chunk := map[string]any{
			"id": resp.ID, "object": "chat.completion.chunk", "created": resp.Created, "model": upstreamModel,
			"choices": []map[string]any{{
				"index":         0,
				"delta":         map[string]any{"role": "assistant", "content": content},
				"finish_reason": nil,
			}},
		}
		b, _ := json.Marshal(chunk)
		out <- StreamChunk{Data: b}
		final := map[string]any{
			"id": resp.ID, "object": "chat.completion.chunk", "created": resp.Created, "model": upstreamModel,
			"choices": []map[string]any{{"index": 0, "delta": map[string]any{}, "finish_reason": "stop"}},
			"usage":   resp.Usage,
		}
		b2, _ := json.Marshal(final)
		out <- StreamChunk{Data: b2}
		out <- StreamChunk{Done: true}
	}()
	return out, nil
}

// --- openai compatible ---

type OpenAICompatibleAdapter struct {
	HTTP *http.Client
}

func NewOpenAICompatible() *OpenAICompatibleAdapter {
	return &OpenAICompatibleAdapter{HTTP: &http.Client{Timeout: 120 * time.Second}}
}

func (a *OpenAICompatibleAdapter) Provider() string { return "openai_compatible" }

func (a *OpenAICompatibleAdapter) Chat(ctx context.Context, ch store.Channel, upstreamModel string, req ChatRequest) (*ChatResponse, error) {
	req.Model = upstreamModel
	if req.WebSearch {
		if !deepSeekSearch(ch) {
			return nil, fmt.Errorf("联网搜索目前只支持 DeepSeek 渠道")
		}
		return a.chatWithWebSearch(ctx, ch, upstreamModel, req)
	}
	req.Stream = false
	req.WebSearch = false
	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, joinURL(ch.BaseURL, "/chat/completions"), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if ch.APIKeyEnc != "" {
		httpReq.Header.Set("Authorization", "Bearer "+ch.APIKeyEnc)
	}
	client := a.clientFor(ch)
	res, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("upstream %d: %s", res.StatusCode, truncate(string(raw), 500))
	}
	var out ChatResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	out.Raw = raw
	if out.Usage.TotalTokens == 0 {
		// 兜底估算
		in := 0
		for _, m := range req.Messages {
			in += estimateTokens(m.Content)
		}
		outContent := ""
		if len(out.Choices) > 0 {
			outContent = out.Choices[0].Message.Content
		}
		out.Usage.PromptTokens = in
		out.Usage.CompletionTokens = estimateTokens(outContent)
		out.Usage.TotalTokens = out.Usage.PromptTokens + out.Usage.CompletionTokens
	}
	return &out, nil
}

func (a *OpenAICompatibleAdapter) ChatStream(ctx context.Context, ch store.Channel, upstreamModel string, req ChatRequest) (<-chan StreamChunk, error) {
	if req.WebSearch {
		resp, err := a.Chat(ctx, ch, upstreamModel, req)
		if err != nil {
			return nil, err
		}
		return streamOneShot(resp), nil
	}
	req.Model = upstreamModel
	req.Stream = true
	req.WebSearch = false
	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, joinURL(ch.BaseURL, "/chat/completions"), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	if ch.APIKeyEnc != "" {
		httpReq.Header.Set("Authorization", "Bearer "+ch.APIKeyEnc)
	}
	client := a.clientFor(ch)
	res, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 300 {
		raw, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		return nil, fmt.Errorf("upstream %d: %s", res.StatusCode, truncate(string(raw), 500))
	}
	out := make(chan StreamChunk, 16)
	go func() {
		defer close(out)
		defer res.Body.Close()
		sc := bufio.NewScanner(res.Body)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			line := sc.Text()
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if payload == "[DONE]" {
				out <- StreamChunk{Done: true}
				return
			}
			out <- StreamChunk{Data: []byte(payload)}
		}
		if err := sc.Err(); err != nil {
			out <- StreamChunk{Err: err}
			return
		}
		out <- StreamChunk{Done: true}
	}()
	return out, nil
}

// ListModelIDs calls the OpenAI-compatible GET /models endpoint.
func (a *OpenAICompatibleAdapter) ListModelIDs(ctx context.Context, ch store.Channel) ([]string, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, joinURL(ch.BaseURL, "/models"), nil)
	if err != nil {
		return nil, err
	}
	if ch.APIKeyEnc != "" {
		httpReq.Header.Set("Authorization", "Bearer "+ch.APIKeyEnc)
	}
	res, err := a.clientFor(ch).Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("upstream %d: %s", res.StatusCode, truncate(string(raw), 500))
	}
	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(payload.Data))
	for _, item := range payload.Data {
		if id := strings.TrimSpace(item.ID); id != "" {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func (a *OpenAICompatibleAdapter) clientFor(ch store.Channel) *http.Client {
	timeout := time.Duration(ch.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	return &http.Client{Timeout: timeout}
}

func joinURL(base, path string) string {
	base = strings.TrimRight(base, "/")
	if strings.HasSuffix(base, "/v1") {
		return base + path
	}
	// 允许用户填 http://host:11434 或 http://host:11434/v1
	if strings.Contains(base, "/v1/") {
		return base + path
	}
	return base + "/v1" + path
}

func lastUserContent(msgs []ChatMessage) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "user" {
			return msgs[i].Content
		}
	}
	if len(msgs) == 0 {
		return ""
	}
	return msgs[len(msgs)-1].Content
}

func estimateTokens(s string) int {
	n := len([]rune(s))
	if n == 0 {
		return 0
	}
	// 粗估：约 2 字符/token（中英混合）
	t := n / 2
	if t < 1 {
		t = 1
	}
	return t
}

// PickProbeModel chooses a real upstream model for a connectivity chat.
// ids comes from GET /models; an empty list falls back to a vendor default.
// Never returns the placeholder name "ping".
func PickProbeModel(provider string, ids []string) string {
	available := map[string]struct{}{}
	var first string
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if first == "" {
			first = id
		}
		available[id] = struct{}{}
	}
	for _, name := range probePreferences(provider) {
		if _, ok := available[name]; ok {
			return name
		}
	}
	if first != "" {
		return first
	}
	prefs := probePreferences(provider)
	if len(prefs) > 0 {
		return prefs[0]
	}
	return "gpt-4o-mini"
}

func probePreferences(provider string) []string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "mock":
		return []string{"mock-chat"}
	case "deepseek":
		return []string{"deepseek-flash", "deepseek-v4-pro", "deepseek-chat", "deepseek-reasoner"}
	case "qwen", "dashscope":
		return []string{"qwen-turbo", "qwen-plus"}
	case "moonshot", "kimi":
		return []string{"moonshot-v1-8k", "moonshot-v1-32k"}
	case "zhipu", "glm":
		return []string{"glm-4-flash", "glm-4"}
	case "openai":
		return []string{"gpt-4o-mini", "gpt-4o"}
	case "ollama":
		return []string{"llama3.2", "qwen2.5"}
	default:
		return nil
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// EstimatePromptTokens exports helper for billing.
func EstimatePromptTokens(msgs []ChatMessage) int {
	n := 0
	for _, m := range msgs {
		n += estimateTokens(m.Content)
	}
	return n
}
