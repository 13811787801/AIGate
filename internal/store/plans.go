package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"aigate/internal/modules"
)

type Plan struct {
	ID        int64
	Code      string
	Name      string
	Modules   map[string]bool
	MaxKeys   int
	Status    int
	Note      string
	CreatedAt string
	UpdatedAt string
}

func (s *Store) SeedPlans() error {
	now := Now()
	plans := []struct {
		Code, Name, Note string
		Mods             map[string]bool
		MaxKeys          int
	}{
		{
			Code: "free", Name: "免费版", Note: "基础调用与操练场",
			MaxKeys: 2,
			Mods: map[string]bool{
				modules.Core: true, modules.Console: true,
				modules.Playground: true, modules.Marketplace: true,
				modules.Wallet: false, modules.AdminChannels: false,
				modules.BillingOnline: false, modules.SSO: false,
				modules.Audit: false, modules.PluginRuntime: false,
			},
		},
		{
			Code: "pro", Name: "专业版", Note: "钱包+渠道+模型管理",
			MaxKeys: 20,
			Mods: map[string]bool{
				modules.Core: true, modules.Console: true,
				modules.Playground: true, modules.Marketplace: true,
				modules.Wallet: true, modules.AdminChannels: true,
				modules.BillingOnline: false, modules.SSO: false,
				modules.Audit: false, modules.PluginRuntime: false,
			},
		},
		{
			Code: "enterprise", Name: "企业版", Note: "全模块（受实例许可证上限约束）",
			MaxKeys: 0,
			Mods: map[string]bool{
				modules.Core: true, modules.Console: true,
				modules.Playground: true, modules.Marketplace: true,
				modules.Wallet: true, modules.AdminChannels: true,
				modules.BillingOnline: true, modules.SSO: true,
				modules.Audit: true, modules.PluginRuntime: true,
			},
		},
	}
	for _, p := range plans {
		b, _ := json.Marshal(p.Mods)
		if err := s.execInsertIgnorePlan(p.Code, p.Name, string(b), p.MaxKeys, p.Note, now); err != nil {
			return err
		}
	}
	// Ensure admin users get enterprise plan if plan_id empty.
	var entID int64
	_ = s.DB.QueryRow(`SELECT id FROM plan WHERE code='enterprise'`).Scan(&entID)
	if entID > 0 {
		_, _ = s.DB.Exec(`UPDATE ` + tblUser + ` SET plan_id=?, role='admin' WHERE (plan_id IS NULL OR plan_id=0) AND (username='admin' OR role='admin')`, entID)
		_, _ = s.DB.Exec(`UPDATE ` + tblUser + ` SET plan_id=(SELECT id FROM plan WHERE code='pro'), role=COALESCE(NULLIF(role,''),'user')
WHERE (plan_id IS NULL OR plan_id=0) AND username!='admin'`)
	}
	return nil
}

func decodeModulesJSON(raw string) map[string]bool {
	out := map[string]bool{}
	_ = json.Unmarshal([]byte(raw), &out)
	return out
}

func (s *Store) ListPlans() ([]Plan, error) {
	rows, err := s.DB.Query(`SELECT id,code,name,modules_json,max_keys,status,COALESCE(note,''),created_at,updated_at FROM plan ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Plan
	for rows.Next() {
		var p Plan
		var raw string
		if err := rows.Scan(&p.ID, &p.Code, &p.Name, &raw, &p.MaxKeys, &p.Status, &p.Note, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		p.Modules = decodeModulesJSON(raw)
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) GetPlan(id int64) (*Plan, error) {
	row := s.DB.QueryRow(`SELECT id,code,name,modules_json,max_keys,status,COALESCE(note,''),created_at,updated_at FROM plan WHERE id=?`, id)
	var p Plan
	var raw string
	if err := row.Scan(&p.ID, &p.Code, &p.Name, &raw, &p.MaxKeys, &p.Status, &p.Note, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	p.Modules = decodeModulesJSON(raw)
	return &p, nil
}

func (s *Store) GetPlanByCode(code string) (*Plan, error) {
	row := s.DB.QueryRow(`SELECT id,code,name,modules_json,max_keys,status,COALESCE(note,''),created_at,updated_at FROM plan WHERE code=?`, code)
	var p Plan
	var raw string
	if err := row.Scan(&p.ID, &p.Code, &p.Name, &raw, &p.MaxKeys, &p.Status, &p.Note, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	p.Modules = decodeModulesJSON(raw)
	return &p, nil
}

func (s *Store) UpsertPlan(code, name, note string, mods map[string]bool, maxKeys, status int) (int64, error) {
	if code == "" || name == "" {
		return 0, fmt.Errorf("code and name required")
	}
	if mods == nil {
		mods = map[string]bool{}
	}
	b, _ := json.Marshal(mods)
	now := Now()
	res, err := s.execUpsertPlan(code, name, string(b), maxKeys, status, note, now)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	if id == 0 {
		_ = s.DB.QueryRow(`SELECT id FROM plan WHERE code=?`, code).Scan(&id)
	}
	return id, nil
}

type AdminUserRow struct {
	ID            int64
	Username      string
	DisplayName   string
	Role          string
	Status        int
	PlanID        sql.NullInt64
	PlanCode      string
	PlanName      string
	Balance       float64
	TotalConsumed float64
	CreatedAt     string
	ParentID      int64
	OrgRole       string
	PermsJSON     string
}

func (s *Store) ListUsersAdmin() ([]AdminUserRow, error) {
	rows, err := s.DB.Query(`
SELECT u.id, u.username, COALESCE(u.display_name,''), COALESCE(u.role,'user'), u.status,
       u.plan_id, COALESCE(p.code,''), COALESCE(p.name,''), u.balance, u.total_consumed, u.created_at,
       COALESCE(u.parent_id,0), COALESCE(u.org_role,'member'), COALESCE(u.perms_json,'')
FROM ` + tblUser + ` u
LEFT JOIN plan p ON p.id = u.plan_id
ORDER BY u.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AdminUserRow
	for rows.Next() {
		var r AdminUserRow
		if err := rows.Scan(&r.ID, &r.Username, &r.DisplayName, &r.Role, &r.Status, &r.PlanID,
			&r.PlanCode, &r.PlanName, &r.Balance, &r.TotalConsumed, &r.CreatedAt, &r.ParentID,
			&r.OrgRole, &r.PermsJSON); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) ListSubAccounts(parentID int64) ([]AdminUserRow, error) {
	rows, err := s.DB.Query(`
SELECT u.id, u.username, COALESCE(u.display_name,''), COALESCE(u.role,'user'), u.status,
       u.plan_id, COALESCE(p.code,''), COALESCE(p.name,''), u.balance, u.total_consumed, u.created_at,
       COALESCE(u.parent_id,0), COALESCE(u.org_role,'member'), COALESCE(u.perms_json,'')
FROM ` + tblUser + ` u
LEFT JOIN plan p ON p.id = u.plan_id
WHERE u.parent_id=? OR u.id=?
ORDER BY CASE WHEN u.id=? THEN 0 ELSE 1 END, u.id`, parentID, parentID, parentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AdminUserRow
	for rows.Next() {
		var r AdminUserRow
		if err := rows.Scan(&r.ID, &r.Username, &r.DisplayName, &r.Role, &r.Status, &r.PlanID,
			&r.PlanCode, &r.PlanName, &r.Balance, &r.TotalConsumed, &r.CreatedAt, &r.ParentID,
			&r.OrgRole, &r.PermsJSON); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type UsageByUser struct {
	UserID       int64
	Username     string
	Calls        int64
	InputTokens  int64
	OutputTokens int64
	Cost         float64
	LastAt       string
}

func (s *Store) UsageByParent(parentID int64) ([]UsageByUser, error) {
	rows, err := s.DB.Query(`
SELECT u.id, u.username,
       COUNT(l.id),
       COALESCE(SUM(l.input_tokens),0),
       COALESCE(SUM(l.output_tokens),0),
       COALESCE(SUM(l.cost),0),
       COALESCE(MAX(l.created_at),'')
FROM ` + tblUser + ` u
LEFT JOIN request_log l ON l.user_id = u.id
WHERE u.parent_id=? OR u.id=?
GROUP BY u.id, u.username
ORDER BY COALESCE(SUM(l.cost),0) DESC`, parentID, parentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []UsageByUser
	for rows.Next() {
		var r UsageByUser
		if err := rows.Scan(&r.UserID, &r.Username, &r.Calls, &r.InputTokens, &r.OutputTokens, &r.Cost, &r.LastAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) CreateUser(username, password, display, role string, planID, parentID int64, balance float64) (int64, error) {
	return s.CreateUserOpts(UserCreateOpts{
		Username: username, Password: password, DisplayName: display,
		Role: role, PlanID: planID, ParentID: parentID, Balance: balance,
		OrgRole: "member",
	})
}

type UserCreateOpts struct {
	Username    string
	Password    string
	DisplayName string
	Role        string
	PlanID      int64
	ParentID    int64
	Balance     float64
	OrgRole     string
	PermsJSON   string
}

func (s *Store) CreateUserOpts(opts UserCreateOpts) (int64, error) {
	username := strings.TrimSpace(opts.Username)
	if username == "" || opts.Password == "" {
		return 0, fmt.Errorf("username and password required")
	}
	role := opts.Role
	if role == "" {
		role = "user"
	}
	orgRole := strings.TrimSpace(opts.OrgRole)
	if orgRole == "" {
		orgRole = "member"
	}
	now := Now()
	res, err := s.DB.Exec(`INSERT INTO `+tblUser+`(username,password_hash,display_name,group_id,status,balance,frozen,total_consumed,plan_id,role,parent_id,org_role,perms_json,created_at,updated_at)
VALUES(?,?,?,1,1,?,0,0,?,?,?,?,?,?,?)`, username, hashPassword(opts.Password), opts.DisplayName, opts.Balance, nullInt64(opts.PlanID), role, nullInt64(opts.ParentID), orgRole, opts.PermsJSON, now, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

type OrgMemberUpdate struct {
	Status      *int
	DisplayName *string
	OrgRole     *string
	PermsJSON   *string
	Password    *string // optional reset
}

func (s *Store) UpdateOrgMember(id int64, upd OrgMemberUpdate) error {
	u, err := s.GetUser(id)
	if err != nil {
		return err
	}
	st := u.Status
	dn := u.DisplayName
	orgRole := u.OrgRole
	if orgRole == "" {
		orgRole = "member"
	}
	perms := u.PermsJSON
	if upd.Status != nil {
		st = *upd.Status
	}
	if upd.DisplayName != nil {
		dn = *upd.DisplayName
	}
	if upd.OrgRole != nil && strings.TrimSpace(*upd.OrgRole) != "" {
		orgRole = strings.TrimSpace(*upd.OrgRole)
	}
	if upd.PermsJSON != nil {
		perms = *upd.PermsJSON
	}
	if upd.Password != nil && *upd.Password != "" {
		_, err = s.DB.Exec(`UPDATE `+tblUser+` SET status=?, display_name=?, org_role=?, perms_json=?, password_hash=?, updated_at=? WHERE id=?`,
			st, dn, orgRole, perms, hashPassword(*upd.Password), Now(), id)
		return err
	}
	_, err = s.DB.Exec(`UPDATE `+tblUser+` SET status=?, display_name=?, org_role=?, perms_json=?, updated_at=? WHERE id=?`,
		st, dn, orgRole, perms, Now(), id)
	return err
}

func nullInt64(v int64) any {
	if v <= 0 {
		return nil
	}
	return v
}

func (s *Store) UpdateUserAdmin(id int64, planID *int64, status *int, role *string, display *string) error {
	u, err := s.GetUser(id)
	if err != nil {
		return err
	}
	plan := u.PlanID
	st := u.Status
	rl := u.Role
	dn := u.DisplayName
	if planID != nil {
		plan = *planID
	}
	if status != nil {
		st = *status
	}
	if role != nil && *role != "" {
		rl = *role
	}
	if display != nil {
		dn = *display
	}
	_, err = s.DB.Exec(`UPDATE ` + tblUser + ` SET plan_id=?, status=?, role=?, display_name=?, updated_at=? WHERE id=?`,
		nullInt64(plan), st, rl, dn, Now(), id)
	return err
}

func (s *Store) WriteAudit(actorID int64, action, target, detail, ip string) {
	_, _ = s.DB.Exec(`INSERT INTO audit_log(actor_user_id,action,target,detail,ip,created_at) VALUES(?,?,?,?,?,?)`,
		nullInt64(actorID), action, target, detail, ip, Now())
}

func (s *Store) CountUsers() (int, error) {
	var n int
	err := s.DB.QueryRow(`SELECT COUNT(1) FROM ` + tblUser).Scan(&n)
	return n, err
}
