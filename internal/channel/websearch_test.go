package channel

import (
	"strings"
	"testing"
)

func TestParseWebSearchResponse(t *testing.T) {
	raw := []byte(`{
		"id":"msg_1","type":"message","stop_reason":"end_turn",
		"content":[
			{"type":"web_search_tool_result","content":[
				{"type":"web_search_result","title":"Reuters","url":"https://www.reuters.com/world/"},
				{"type":"web_search_result","title":"Reuters","url":"https://www.reuters.com/world/"}
			]},
			{"type":"text","text":"今日要闻摘要。"}
		],
		"usage":{"input_tokens":20,"output_tokens":30}
	}`)
	out, err := parseWebSearchResponse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if out.Usage.TotalTokens != 50 {
		t.Fatalf("usage %+v", out.Usage)
	}
	got := out.Choices[0].Message.Content
	if !strings.Contains(got, "今日要闻摘要") || !strings.Contains(got, "https://www.reuters.com/world/") {
		t.Fatalf("content %q", got)
	}
	if strings.Count(got, "https://www.reuters.com/world/") != 1 {
		t.Fatalf("duplicate source: %q", got)
	}
}

func TestDeepSeekMessagesURL(t *testing.T) {
	got := deepSeekMessagesURL("https://api.deepseek.com/v1")
	if got != "https://api.deepseek.com/anthropic/v1/messages" {
		t.Fatalf("got %s", got)
	}
}
