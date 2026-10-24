package orgperm

import (
	"encoding/json"
	"strings"

	"aigate/internal/modules"
)

const (
	RoleMember    = "member"
	RoleDeveloper = "developer"
	RoleOperator  = "operator"
	RoleCustom    = "custom"
)

// AssignableModules can be toggled on the org admin console.
var AssignableModules = []string{
	modules.Marketplace,
	modules.Playground,
	modules.Wallet,
	modules.AdminChannels,
	modules.Audit,
}

type RoleInfo struct {
	Code        string          `json:"code"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Modules     map[string]bool `json:"modules"`
}

func Catalog() []RoleInfo {
	return []RoleInfo{
		{Code: RoleMember, Name: "普通成员", Description: "广场 / 操练场 / 钱包（在天花板内）", Modules: Preset(RoleMember)},
		{Code: RoleDeveloper, Name: "开发者", Description: "联调常用：广场、操练场、钱包", Modules: Preset(RoleDeveloper)},
		{Code: RoleOperator, Name: "运维", Description: "含渠道与模型管理（admin_channels）", Modules: Preset(RoleOperator)},
		{Code: RoleCustom, Name: "自定义", Description: "仅按勾选模块授权", Modules: map[string]bool{}},
	}
}

func NormalizeRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case RoleDeveloper:
		return RoleDeveloper
	case RoleOperator:
		return RoleOperator
	case RoleCustom:
		return RoleCustom
	default:
		return RoleMember
	}
}

func Preset(orgRole string) map[string]bool {
	out := map[string]bool{
		modules.Core:    true,
		modules.Console: true,
	}
	switch NormalizeRole(orgRole) {
	case RoleOperator:
		out[modules.Marketplace] = true
		out[modules.Playground] = true
		out[modules.Wallet] = true
		out[modules.AdminChannels] = true
	case RoleDeveloper:
		out[modules.Marketplace] = true
		out[modules.Playground] = true
		out[modules.Wallet] = true
	case RoleCustom:
		// empty beyond core/console; filled from perms_json
	default: // member
		out[modules.Marketplace] = true
		out[modules.Playground] = true
		out[modules.Wallet] = true
	}
	return out
}

func ParsePermsJSON(raw string) map[string]bool {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return nil
	}
	var m map[string]bool
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil
	}
	return m
}

func EncodePermsJSON(m map[string]bool) string {
	if m == nil {
		return ""
	}
	// Keep only assignable keys for storage clarity.
	clean := map[string]bool{}
	for _, id := range AssignableModules {
		if m[id] {
			clean[id] = true
		}
	}
	if len(clean) == 0 {
		return "{}"
	}
	b, _ := json.Marshal(clean)
	return string(b)
}

// Resolve builds org ACL then intersects with ceiling (never exceed).
func Resolve(orgRole, permsJSON string, ceiling map[string]bool) map[string]bool {
	role := NormalizeRole(orgRole)
	base := Preset(role)
	if role == RoleCustom {
		base = map[string]bool{modules.Core: true, modules.Console: true}
	}
	if custom := ParsePermsJSON(permsJSON); custom != nil {
		if role == RoleCustom {
			for _, id := range AssignableModules {
				base[id] = custom[id]
			}
		} else {
			// Non-custom: perms_json acts as optional overlay (explicit false/true).
			for k, v := range custom {
				base[k] = v
			}
			base[modules.Core] = true
			base[modules.Console] = true
		}
	}
	return intersect(ceiling, base)
}

func intersect(ceiling, org map[string]bool) map[string]bool {
	out := map[string]bool{
		modules.Core:    true,
		modules.Console: true,
	}
	all := []string{
		modules.Marketplace, modules.Wallet, modules.Playground, modules.AdminChannels,
		modules.BillingOnline, modules.SSO, modules.Audit, modules.PluginRuntime,
	}
	for _, id := range all {
		cOK := true
		if ceiling != nil {
			if v, ok := ceiling[id]; ok {
				cOK = v
			} else {
				cOK = false
			}
		}
		oOK := false
		if org != nil {
			oOK = org[id]
		}
		out[id] = cOK && oOK
	}
	return out
}
