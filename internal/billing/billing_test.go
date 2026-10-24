package billing

import (
	"database/sql"
	"testing"

	"aigate/internal/store"
)

func TestEstimateToken(t *testing.T) {
	s := &Service{FreezeMargin: 1.2}
	m := &store.ModelConfig{
		BillingMode:      "TOKEN",
		InputPricePer1K:  1,
		OutputPricePer1K: 2,
		MaxTokens:        1000,
	}
	est := s.Estimate(m, 1.0, 1000)
	// in 0.5 + out 2.0 = 2.5 * 1.2 = 3.0
	if est < 2.9 || est > 3.1 {
		t.Fatalf("unexpected estimate %v", est)
	}
}

func TestActualToken(t *testing.T) {
	s := &Service{}
	m := &store.ModelConfig{BillingMode: "TOKEN", InputPricePer1K: 1, OutputPricePer1K: 2}
	got := s.Actual(m, 1.0, 1000, 500)
	// 1 + 1 = 2
	if got != 2 {
		t.Fatalf("got %v", got)
	}
}

func TestEstimateRequest(t *testing.T) {
	s := &Service{FreezeMargin: 2}
	m := &store.ModelConfig{
		BillingMode: "REQUEST",
		PricePerRequest: sql.NullFloat64{Float64: 0.1, Valid: true},
	}
	est := s.Estimate(m, 1, 0)
	if est != 0.2 {
		t.Fatalf("got %v", est)
	}
}
