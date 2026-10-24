package entitlement

import (
	"aigate/internal/license"
	"aigate/internal/modules"
	"aigate/internal/orgperm"
	"aigate/internal/store"
)

// Service resolves per-user module rights.
// Authority is ALWAYS server-side: client UI flags are hints only.
type Service struct {
	Store    *store.Store
	Instance modules.Flags
	License  license.State
}

func Intersect(ceiling, plan map[string]bool) map[string]bool {
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
		pOK := true
		if plan != nil {
			if v, ok := plan[id]; ok {
				pOK = v
			} else {
				pOK = false
			}
		}
		out[id] = cOK && pOK
	}
	return out
}

func (s *Service) InstanceCeiling() map[string]bool {
	base := s.Instance.Map()
	if s.License.Loaded && s.License.Valid && s.License.Ceiling != nil {
		return Intersect(base, s.License.Ceiling)
	}
	if s.License.Loaded && !s.License.Valid {
		// Invalid license: hard lock to core only.
		return map[string]bool{modules.Core: true, modules.Console: true}
	}
	return base
}

type Snapshot struct {
	Modules    map[string]bool `json:"modules"`
	PlanID     int64           `json:"plan_id"`
	PlanCode   string          `json:"plan_code"`
	PlanName   string          `json:"plan_name"`
	IsAdmin    bool            `json:"is_admin"`
	OrgRole    string          `json:"org_role,omitempty"`
	LicenseOK  bool            `json:"license_ok"`
	LicenseMsg string          `json:"license_msg"`
	MaxUsers   int             `json:"max_users"`
}

func (s *Service) ForUser(u *store.User, isAdmin bool) Snapshot {
	ceiling := s.InstanceCeiling()
	snap := Snapshot{
		IsAdmin:    isAdmin,
		LicenseOK:  !s.License.Loaded || s.License.Valid,
		LicenseMsg: s.License.Reason,
		MaxUsers:   s.License.MaxUsers,
		Modules:    ceiling,
		OrgRole:    u.OrgRole,
	}
	if isAdmin {
		// Admins see full instance ceiling (still cannot exceed license).
		snap.Modules = ceiling
		if u.PlanID > 0 {
			if p, err := s.Store.GetPlan(u.PlanID); err == nil {
				snap.PlanID, snap.PlanCode, snap.PlanName = p.ID, p.Code, p.Name
			}
		}
		return snap
	}
	planMods := ceiling
	if u.PlanID > 0 {
		if p, err := s.Store.GetPlan(u.PlanID); err == nil && p.Status == 1 {
			snap.PlanID, snap.PlanCode, snap.PlanName = p.ID, p.Code, p.Name
			planMods = p.Modules
		}
	} else if p, err := s.Store.GetPlanByCode("free"); err == nil {
		snap.PlanID, snap.PlanCode, snap.PlanName = p.ID, p.Code, p.Name
		planMods = p.Modules
	}
	afterPlan := Intersect(ceiling, planMods)
	// Org ACL only for sub-accounts; top-level users keep plan ∩ ceiling.
	if u.ParentID > 0 {
		snap.Modules = orgperm.Resolve(u.OrgRole, u.PermsJSON, afterPlan)
	} else {
		snap.Modules = afterPlan
	}
	return snap
}

func (s *Service) Enabled(u *store.User, isAdmin bool, moduleID string) bool {
	if moduleID == modules.Core || moduleID == modules.Console {
		return true
	}
	return s.ForUser(u, isAdmin).Modules[moduleID]
}
