package store

import (
	"path/filepath"
	"testing"
)

func TestUpdateModelKeepsID(t *testing.T) {
	dir := t.TempDir()
	s, err := Open("sqlite", filepath.Join(dir, "t.db"), "")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	chID, err := s.CreateChannel(Channel{Name: "ds", Provider: "deepseek", BaseURL: "https://api.deepseek.com/v1", Status: 1, Weight: 100, TimeoutMs: 30000})
	if err != nil {
		t.Fatal(err)
	}
	modelID, err := s.CreateModel(ModelConfig{
		ModelName: "draft", DisplayName: "", BillingMode: "TOKEN",
		InputPricePer1K: 0.001, OutputPricePer1K: 0.002, MaxTokens: 1024, Enabled: true, SupportsStream: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.BindModelChannel(modelID, chID, "ping", 100); err != nil {
		t.Fatal(err)
	}

	if err := s.UpdateModel(ModelConfig{
		ID: modelID, ModelName: "deepseek-flash", DisplayName: "DeepSeek Flash",
		BillingMode: "TOKEN", InputPricePer1K: 0.001, OutputPricePer1K: 0.002, MaxTokens: 8192,
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetModelPrimaryChannel(modelID, chID, "deepseek-flash"); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetModel(modelID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ModelName != "deepseek-flash" || got.DisplayName != "DeepSeek Flash" {
		t.Fatalf("model = %+v", got)
	}
	sum, err := s.ListModelBindingSummary()
	if err != nil {
		t.Fatal(err)
	}
	if sum[modelID].Upstream != "deepseek-flash" || sum[modelID].ChannelID != chID {
		t.Fatalf("binding = %+v", sum[modelID])
	}

	other, err := s.CreateModel(ModelConfig{ModelName: "other", DisplayName: "other", BillingMode: "TOKEN", MaxTokens: 100, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	err = s.UpdateModel(ModelConfig{ID: other, ModelName: "deepseek-flash", DisplayName: "dup", BillingMode: "TOKEN", MaxTokens: 100})
	if err != ErrDuplicate {
		t.Fatalf("dup err = %v", err)
	}
}
