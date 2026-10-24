package entitlement

import (
	"testing"

	"aigate/internal/modules"
)

func TestIntersect(t *testing.T) {
	ceiling := map[string]bool{
		modules.Wallet: true, modules.Playground: true, modules.Marketplace: false,
	}
	plan := map[string]bool{
		modules.Wallet: false, modules.Playground: true, modules.Marketplace: true,
	}
	out := Intersect(ceiling, plan)
	if out[modules.Core] != true || out[modules.Console] != true {
		t.Fatal("core/console must be on")
	}
	if out[modules.Wallet] {
		t.Fatal("wallet should be off (plan denies)")
	}
	if !out[modules.Playground] {
		t.Fatal("playground should be on")
	}
	if out[modules.Marketplace] {
		t.Fatal("marketplace should be off (ceiling denies)")
	}
}
