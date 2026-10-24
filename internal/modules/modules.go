package modules

// Feature module IDs for pluggable commercial SKUs.
// Core gateway always runs; optional modules gate UI/API surfaces.
const (
	Core           = "core"
	Console        = "console"
	Marketplace    = "marketplace"
	Wallet         = "wallet"
	Playground     = "playground"
	AdminChannels  = "admin_channels"
	BillingOnline  = "billing_online"
	SSO            = "sso"
	Audit          = "audit"
	PluginRuntime  = "plugin_runtime"
)

// Flags mirrors config.modules.*
type Flags struct {
	Marketplace   bool `yaml:"marketplace" json:"marketplace"`
	Wallet        bool `yaml:"wallet" json:"wallet"`
	Playground    bool `yaml:"playground" json:"playground"`
	AdminChannels bool `yaml:"admin_channels" json:"admin_channels"`
	BillingOnline bool `yaml:"billing_online" json:"billing_online"`
	SSO           bool `yaml:"sso" json:"sso"`
	Audit         bool `yaml:"audit" json:"audit"`
	PluginRuntime bool `yaml:"plugin_runtime" json:"plugin_runtime"`
}

func Default() Flags {
	return Flags{
		Marketplace:   true,
		Wallet:        true,
		Playground:    true,
		AdminChannels: true,
		BillingOnline: false,
		SSO:           false,
		Audit:         false,
		PluginRuntime: false,
	}
}

func (f Flags) Enabled(id string) bool {
	switch id {
	case Core, Console:
		return true
	case Marketplace:
		return f.Marketplace
	case Wallet:
		return f.Wallet
	case Playground:
		return f.Playground
	case AdminChannels:
		return f.AdminChannels
	case BillingOnline:
		return f.BillingOnline
	case SSO:
		return f.SSO
	case Audit:
		return f.Audit
	case PluginRuntime:
		return f.PluginRuntime
	default:
		return false
	}
}

func (f Flags) Map() map[string]bool {
	return map[string]bool{
		Core:          true,
		Console:       true,
		Marketplace:   f.Marketplace,
		Wallet:        f.Wallet,
		Playground:    f.Playground,
		AdminChannels: f.AdminChannels,
		BillingOnline: f.BillingOnline,
		SSO:           f.SSO,
		Audit:         f.Audit,
		PluginRuntime: f.PluginRuntime,
	}
}
