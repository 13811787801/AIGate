package channel

import "testing"

func TestPickProbeModelDeepSeek(t *testing.T) {
	got := PickProbeModel("deepseek", []string{"deepseek-v4-pro", "deepseek-flash"})
	if got != "deepseek-flash" {
		t.Fatalf("got %q", got)
	}
}

func TestPickProbeModelUsesFirstWhenNoPreference(t *testing.T) {
	got := PickProbeModel("custom", []string{"my-model"})
	if got != "my-model" {
		t.Fatalf("got %q", got)
	}
}

func TestPickProbeModelNeverPing(t *testing.T) {
	got := PickProbeModel("deepseek", nil)
	if got == "ping" || got == "" {
		t.Fatalf("got %q", got)
	}
	if got != "deepseek-flash" {
		t.Fatalf("got %q", got)
	}
}
