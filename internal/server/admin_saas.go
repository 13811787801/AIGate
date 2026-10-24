package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"aigate/internal/authtoken"
	"aigate/internal/modules"
	"aigate/internal/orgperm"
	"aigate/internal/store"
)

func (a *App) isAdminUser(username string) bool {
	if username == a.Cfg.Auth.AdminUsername || username == "admin" {
		return true
	}
	u, err := a.Store.GetUserByUsername(username)
	if err != nil {
		return false
	}
	return u.Role == "admin"
}

func (a *App) requireAdminJWT(w http.ResponseWriter, r *http.Request) (*authtoken.Claims, bool) {
	claims, err := a.requireJWT(r)
	if err != nil || !claims.IsAdmin {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "admin only"})
		return nil, false
	}
	// Re-check role from DB (JWT is_admin can be stale / forged if secret leaked — still verify).
	u, err := a.Store.GetUser(claims.UserID)
	if err != nil || u.Status != 1 || (u.Role != "admin" && !a.isAdminUser(u.Username)) {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "admin role required"})
		return nil, false
	}
	return claims, true
}

func clientIPFrom(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	return r.RemoteAddr
}

// requireModuleFor enforces entitlements from DB+license on every call (never trust Electron/web).
func (a *App) requireModuleFor(w http.ResponseWriter, r *http.Request, claims *authtoken.Claims, id string) bool {
	u, err := a.Store.GetUser(claims.UserID)
	if err != nil || u.Status != 1 {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "user disabled or missing"})
		return false
	}
	isAdmin := claims.IsAdmin || u.Role == "admin" || a.isAdminUser(u.Username)
	if a.Ent.Enabled(u, isAdmin, id) {
		return true
	}
	a.Store.WriteAudit(claims.UserID, "module_denied", id, "forbidden", clientIPFrom(r))
	writeJSON(w, http.StatusForbidden, map[string]any{
		"error":  "module not entitled",
		"module": id,
		"hint":   "当前套餐未包含此功能，请联系管理员升级",
	})
	return false
}

func (a *App) isSaasMode() bool {
	return strings.EqualFold(a.Cfg.Deploy.Mode, "saas")
}

func (a *App) adminSurface() string {
	if a.isSaasMode() {
		return "vendor"
	}
	return "org"
}

func (a *App) sessionSnapshot(claims *authtoken.Claims) (map[string]any, error) {
	u, err := a.Store.GetUser(claims.UserID)
	if err != nil {
		return nil, err
	}
	isAdmin := claims.IsAdmin || u.Role == "admin" || a.isAdminUser(u.Username)
	snap := a.Ent.ForUser(u, isAdmin)
	// SaaS: refuse session if license invalid when require or always warn strongly
	licenseBlock := a.isSaasMode() && a.Lic.Loaded && !a.Lic.Valid
	return map[string]any{
		"user": map[string]any{
			"id": u.ID, "username": u.Username, "display_name": u.DisplayName,
			"is_admin": isAdmin, "role": u.Role, "balance": u.Balance, "frozen": u.Frozen,
			"total_consumed": u.TotalConsumed, "group_id": u.GroupID, "plan_id": u.PlanID,
			"parent_id": u.ParentID, "org_role": u.OrgRole,
		},
		"entitlements":   snap,
		"deploy_mode":    a.Cfg.Deploy.Mode,
		"admin_surface":  a.adminSurface(),
		"product_name":   a.Cfg.Deploy.ProductName,
		"modules":        snap.Modules,
		"license_block":  licenseBlock,
	}, nil
}

func (a *App) handleSession(w http.ResponseWriter, r *http.Request) {
	claims, err := a.requireJWT(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	out, err := a.sessionSnapshot(claims)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "user missing"})
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *App) handleAdminPlans(w http.ResponseWriter, r *http.Request) {
	claims, ok := a.requireAdminJWT(w, r)
	if !ok {
		return
	}
	// 私有化：屏蔽厂商套餐运营台（持续经营分界）
	if !a.isSaasMode() {
		if r.Method == http.MethodGet {
			// 只读回显许可证/实例上限，禁止编辑 SKU
			writeJSON(w, http.StatusOK, map[string]any{
				"items":          []any{},
				"catalog":        []string{},
				"read_only":      true,
				"admin_surface":  "org",
				"hint":           "私有化模式不开放厂商套餐编辑；请使用组织管理查看子账户用量",
				"instance_modules": a.Ent.InstanceCeiling(),
			})
			return
		}
		writeJSON(w, http.StatusForbidden, map[string]any{
			"error": "plan editing disabled in onprem mode",
			"hint":  "私有化交付由许可证锁定模块；日常请用组织管理",
		})
		return
	}
	switch r.Method {
	case http.MethodGet:
		list, err := a.Store.ListPlans()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		items := make([]map[string]any, 0, len(list))
		for _, p := range list {
			items = append(items, map[string]any{
				"id": p.ID, "code": p.Code, "name": p.Name, "modules": p.Modules,
				"max_keys": p.MaxKeys, "status": p.Status, "note": p.Note,
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"items": items,
			"catalog": []string{
				modules.Marketplace, modules.Wallet, modules.Playground, modules.AdminChannels,
				modules.BillingOnline, modules.SSO, modules.Audit, modules.PluginRuntime,
			},
		})
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		var body struct {
			Code    string          `json:"code"`
			Name    string          `json:"name"`
			Note    string          `json:"note"`
			Modules map[string]bool `json:"modules"`
			MaxKeys int             `json:"max_keys"`
			Status  int             `json:"status"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad json"})
			return
		}
		if body.Status == 0 {
			body.Status = 1
		}
		id, err := a.Store.UpsertPlan(body.Code, body.Name, body.Note, body.Modules, body.MaxKeys, body.Status)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		a.Store.WriteAudit(claims.UserID, "plan_upsert", body.Code, body.Name, clientIPFrom(r))
		writeJSON(w, http.StatusOK, map[string]any{"id": id, "ok": true})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "GET/POST only"})
	}
}

func (a *App) handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	claims, ok := a.requireAdminJWT(w, r)
	if !ok {
		return
	}
	if !a.isSaasMode() {
		a.handleOrgUsers(w, r, claims)
		return
	}
	switch r.Method {
	case http.MethodGet:
		list, err := a.Store.ListUsersAdmin()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		items := make([]map[string]any, 0, len(list))
		for _, u := range list {
			var planID any
			if u.PlanID.Valid {
				planID = u.PlanID.Int64
			}
			items = append(items, map[string]any{
				"id": u.ID, "username": u.Username, "display_name": u.DisplayName,
				"role": u.Role, "status": u.Status, "plan_id": planID,
				"plan_code": u.PlanCode, "plan_name": u.PlanName,
				"balance": u.Balance, "total_consumed": u.TotalConsumed, "created_at": u.CreatedAt,
				"parent_id": u.ParentID,
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items, "admin_surface": "vendor"})
	case http.MethodPost:
		var body struct {
			Username    string  `json:"username"`
			Password    string  `json:"password"`
			DisplayName string  `json:"display_name"`
			Role        string  `json:"role"`
			PlanID      int64   `json:"plan_id"`
			Balance     float64 `json:"balance"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad json"})
			return
		}
		if a.Lic.Valid && a.Lic.MaxUsers > 0 {
			n, _ := a.Store.CountUsers()
			if n >= a.Lic.MaxUsers {
				writeJSON(w, http.StatusForbidden, map[string]any{"error": "license max_users reached"})
				return
			}
		}
		id, err := a.Store.CreateUser(body.Username, body.Password, body.DisplayName, body.Role, body.PlanID, 0, body.Balance)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		a.Store.WriteAudit(claims.UserID, "user_create", body.Username, strconv.FormatInt(id, 10), clientIPFrom(r))
		writeJSON(w, http.StatusOK, map[string]any{"id": id, "ok": true})
	case http.MethodPatch:
		var body struct {
			ID          int64   `json:"id"`
			PlanID      *int64  `json:"plan_id"`
			Status      *int    `json:"status"`
			Role        *string `json:"role"`
			DisplayName *string `json:"display_name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ID <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "id required"})
			return
		}
		if err := a.Store.UpdateUserAdmin(body.ID, body.PlanID, body.Status, body.Role, body.DisplayName); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		a.Store.WriteAudit(claims.UserID, "user_update", strconv.FormatInt(body.ID, 10), "", clientIPFrom(r))
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "GET/POST/PATCH only"})
	}
}

func (a *App) handleOrgUsers(w http.ResponseWriter, r *http.Request, claims *authtoken.Claims) {
	switch r.Method {
	case http.MethodGet:
		list, err := a.Store.ListSubAccounts(claims.UserID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		ceiling := a.Ent.InstanceCeiling()
		items := make([]map[string]any, 0, len(list))
		for _, u := range list {
			full, _ := a.Store.GetUser(u.ID)
			isSelf := u.ID == claims.UserID
			isAdminRow := u.Role == "admin" || a.isAdminUser(u.Username)
			var effective map[string]bool
			if full != nil {
				effective = a.Ent.ForUser(full, isAdminRow || isSelf && claims.IsAdmin).Modules
			}
			perms := orgperm.ParsePermsJSON(u.PermsJSON)
			if perms == nil {
				perms = map[string]bool{}
			}
			items = append(items, map[string]any{
				"id": u.ID, "username": u.Username, "display_name": u.DisplayName,
				"role": u.Role, "status": u.Status, "balance": u.Balance,
				"total_consumed": u.TotalConsumed, "created_at": u.CreatedAt,
				"parent_id": u.ParentID, "is_self": isSelf,
				"org_role": orgperm.NormalizeRole(u.OrgRole), "perms": perms,
				"effective_modules": effective, "ceiling": ceiling,
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"items": items, "admin_surface": "org",
			"assignable_modules": orgperm.AssignableModules,
			"roles":              orgperm.Catalog(),
		})
	case http.MethodPost:
		var body struct {
			Username    string          `json:"username"`
			Password    string          `json:"password"`
			DisplayName string          `json:"display_name"`
			Balance     float64         `json:"balance"`
			OrgRole     string          `json:"org_role"`
			Perms       map[string]bool `json:"perms"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad json"})
			return
		}
		if a.Lic.Valid && a.Lic.MaxUsers > 0 {
			n, _ := a.Store.CountUsers()
			if n >= a.Lic.MaxUsers {
				writeJSON(w, http.StatusForbidden, map[string]any{"error": "license max_users reached"})
				return
			}
		}
		me, err := a.Store.GetUser(claims.UserID)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "user missing"})
			return
		}
		orgRole := orgperm.NormalizeRole(body.OrgRole)
		permsJSON := orgperm.EncodePermsJSON(body.Perms)
		id, err := a.Store.CreateUserOpts(store.UserCreateOpts{
			Username: body.Username, Password: body.Password, DisplayName: body.DisplayName,
			Role: "user", PlanID: me.PlanID, ParentID: claims.UserID, Balance: body.Balance,
			OrgRole: orgRole, PermsJSON: permsJSON,
		})
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		a.Store.WriteAudit(claims.UserID, "org_subuser_create", body.Username, orgRole+":"+strconv.FormatInt(id, 10), clientIPFrom(r))
		writeJSON(w, http.StatusOK, map[string]any{"id": id, "ok": true, "org_role": orgRole})
	case http.MethodPatch:
		var body struct {
			ID          int64           `json:"id"`
			Status      *int            `json:"status"`
			DisplayName *string         `json:"display_name"`
			OrgRole     *string         `json:"org_role"`
			Perms       map[string]bool `json:"perms"`
			Password    *string         `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ID <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "id required"})
			return
		}
		u, err := a.Store.GetUser(body.ID)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
			return
		}
		if u.ParentID != claims.UserID && u.ID != claims.UserID {
			writeJSON(w, http.StatusForbidden, map[string]any{"error": "not your sub-account"})
			return
		}
		if u.ID == claims.UserID {
			// Root admin may rename self display, but not demote/disable self via org ACL.
			if body.Status != nil && *body.Status == 0 {
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": "cannot disable yourself"})
				return
			}
			if body.OrgRole != nil || body.Perms != nil {
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": "cannot change own org permissions here"})
				return
			}
		}
		upd := store.OrgMemberUpdate{
			Status: body.Status, DisplayName: body.DisplayName, Password: body.Password,
		}
		if body.OrgRole != nil {
			nr := orgperm.NormalizeRole(*body.OrgRole)
			upd.OrgRole = &nr
		}
		if body.Perms != nil {
			pj := orgperm.EncodePermsJSON(body.Perms)
			upd.PermsJSON = &pj
		}
		if err := a.Store.UpdateOrgMember(body.ID, upd); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		a.Store.WriteAudit(claims.UserID, "org_subuser_update", strconv.FormatInt(body.ID, 10), "", clientIPFrom(r))
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "GET/POST/PATCH only"})
	}
}

func (a *App) handleOrgRoles(w http.ResponseWriter, r *http.Request) {
	if _, err := a.requireJWT(r); err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	if a.isSaasMode() {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "org roles are for onprem mode"})
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "GET only"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"roles":              orgperm.Catalog(),
		"assignable_modules": orgperm.AssignableModules,
		"module_labels": map[string]string{
			modules.Marketplace:   "模型广场",
			modules.Playground:    "操练场",
			modules.Wallet:        "钱包",
			modules.AdminChannels: "渠道与模型管理",
			modules.Audit:         "审计",
		},
		"ceiling": a.Ent.InstanceCeiling(),
	})
}

func (a *App) handleOrgUsage(w http.ResponseWriter, r *http.Request) {
	claims, ok := a.requireAdminJWT(w, r)
	if !ok {
		return
	}
	if a.isSaasMode() {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "org usage is for onprem mode"})
		return
	}
	list, err := a.Store.UsageByParent(claims.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	items := make([]map[string]any, 0, len(list))
	for _, u := range list {
		items = append(items, map[string]any{
			"user_id": u.UserID, "username": u.Username, "calls": u.Calls,
			"input_tokens": u.InputTokens, "output_tokens": u.OutputTokens,
			"cost": u.Cost, "last_at": u.LastAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "admin_surface": "org"})
}
