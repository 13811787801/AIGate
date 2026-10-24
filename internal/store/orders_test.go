package store

import "testing"

func TestMockPlanPrice(t *testing.T) {
	if MockPlanPrice("pro") != 99 {
		t.Fatalf("pro")
	}
	if MockPlanPrice("enterprise") != 999 {
		t.Fatalf("enterprise")
	}
	if MockPlanPrice("free") != 0 {
		t.Fatalf("free")
	}
	if MockPlanPrice("nope") >= 0 {
		t.Fatalf("unknown should be <0")
	}
}
