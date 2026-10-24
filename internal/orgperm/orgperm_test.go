package orgperm

import (
	"testing"

	"aigate/internal/modules"
)

func TestPresetOperatorHasChannels(t *testing.T) {
	p := Preset(RoleOperator)
	if !p[modules.AdminChannels] {
		t.Fatal("operator should have admin_channels")
	}
	if Preset(RoleMember)[modules.AdminChannels] {
		t.Fatal("member should not have admin_channels")
	}
}

func TestResolveCannotExceedCeiling(t *testing.T) {
	ceiling := map[string]bool{
		modules.Core: true, modules.Console: true,
		modules.Marketplace: true, modules.Playground: true,
		modules.Wallet: false, modules.AdminChannels: false,
	}
	got := Resolve(RoleOperator, "", ceiling)
	if got[modules.AdminChannels] {
		t.Fatal("admin_channels must stay false when ceiling denies")
	}
	if !got[modules.Marketplace] {
		t.Fatal("marketplace should remain")
	}
}

func TestCustomPerms(t *testing.T) {
	ceiling := map[string]bool{
		modules.Core: true, modules.Console: true,
		modules.Playground: true, modules.Marketplace: true,
		modules.Wallet: true, modules.AdminChannels: true,
	}
	got := Resolve(RoleCustom, `{"playground":true}`, ceiling)
	if !got[modules.Playground] {
		t.Fatal("playground")
	}
	if got[modules.Marketplace] || got[modules.AdminChannels] {
		t.Fatal("custom should only grant checked modules")
	}
}
