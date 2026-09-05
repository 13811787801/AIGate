package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"aigate/internal/auth"
	"aigate/internal/authtoken"
	"aigate/internal/channel"
	"aigate/internal/store"
	"aigate/pkg/trace"
)

func (a *App) isAdminUser(username string) bool {
	return username == a.Cfg.Auth.AdminUsername || username == "admin"
}

func (a *App) requireJWT(r *http.Request) (*authtoken.Claims, error) {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(strings.ToLower(h), "bearer ") {
		return nil, errors.New("missing bearer token")
	}
	tok := strings.TrimSpace(h[7:])
	return authtoken.Parse(a.Cfg.Auth.JWTSecret, tok)
}

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST only"})
		return
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad json"})
		return
	}
	u, err := a.Store.GetUserByUsername(body.Username)
	if err != nil || !a.Store.VerifyPassword(u, body.Password) || u.Status != 1 {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "invalid username or password"})
		return
	}
	isAdmin := a.isAdminUser(u.Username)
	token, err := authtoken.Issue(a.Cfg.Auth.JWTSecret, u.ID, u.Username, isAdmin, 0)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"token": token,
		"user": map[string]any{
			"id": u.ID, "username": u.Username, "display_name": u.DisplayName,
			"is_admin": isAdmin, "balance": u.Balance, "frozen": u.Frozen, "total_consumed": u.TotalConsumed,
		},
	})
}

func (a *App) handleMe(w http.ResponseWriter, r *http.Request) {
	claims, err := a.requireJWT(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	u, err := a.Store.GetUser(claims.UserID)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "user missing"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id": u.ID, "username": u.Username, "display_name": u.DisplayName,
		"is_admin": a.isAdminUser(u.Username), "balance": u.Balance, "frozen": u.Frozen, "total_consumed": u.TotalConsumed,
		"group_id": u.GroupID,
	})
}

func (a *App) handleConsoleWallet(w http.ResponseWriter, r *http.Request) {
	claims, err := a.requireJWT(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	u, err := a.Store.GetUser(claims.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"balance": u.Balance, "frozen": u.Frozen, "total_consumed": u.TotalConsumed,
	})
}

func (a *App) handleConsoleKeys(w http.ResponseWriter, r *http.Request) {
	claims, err := a.requireJWT(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		keys, err := a.Store.ListAPIKeysByUser(claims.UserID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		items := make([]map[string]any, 0, len(keys))
		for _, k := range keys {
			items = append(items, map[string]any{
				"id": k.ID, "name": k.Name, "key_prefix": k.KeyPrefix + "...",
				"status": k.Status, "rpm_limit": k.RPMLimit, "tpm_limit": k.TPMLimit,
				"quota_used": k.QuotaUsed,
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	case http.MethodPost:
		var body struct {
			Name string `json:"name"`
			RPM  int    `json:"rpm_limit"`
			TPM  int    `json:"tpm_limit"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		plain, key, err := a.Store.CreateUserAPIKey(claims.UserID, body.Name, body.RPM, body.TPM)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"id": key.ID, "name": key.Name, "api_key": plain, // 仅创建时返回明文
			"hint": "请立即保存，系统只存储哈希，无法再次查看完整密钥",
		})
	case http.MethodDelete:
		idStr := r.URL.Query().Get("id")
		id, _ := strconv.ParseInt(idStr, 10, 64)
		if id <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "id required"})
			return
		}
		if err := a.Store.DeleteAPIKey(claims.UserID, id); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
	}
}

func (a *App) handleConsoleLogs(w http.ResponseWriter, r *http.Request) {
	claims, err := a.requireJWT(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	logs, err := a.Store.ListLogsByUser(claims.UserID, 100)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": logs})
}

func (a *App) handleConsoleModels(w http.ResponseWriter, r *http.Request) {
	claims, err := a.requireJWT(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	u, err := a.Store.GetUser(claims.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	models, err := a.Store.ListEnabledModelsForGroup(u.GroupID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": models})
}

func (a *App) handleConsoleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST only"})
		return
	}
	claims, err := a.requireJWT(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad body"})
		return
	}
	var req channel.ChatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad json"})
		return
	}
	req.Stream = false

	keys, err := a.Store.ListAPIKeysByUser(claims.UserID)
	if err != nil || len(keys) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "请先创建至少一个 API 令牌"})
		return
	}
	// 用用户自己的第一条启用 key 走网关内核（通过伪造 Bearer 复用 handleChat 逻辑太重，这里直接调内部）
	k := keys[0]
	for _, item := range keys {
		if item.Status == 1 {
			k = item
			break
		}
	}
	u, err := a.Store.GetUser(claims.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	groupID := u.GroupID
	if k.GroupID.Valid {
		groupID = k.GroupID.Int64
	}
	g, err := a.Store.GetGroup(groupID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	p := &auth.Principal{Key: &k, User: u, Group: g, GroupID: groupID, ClientIP: r.RemoteAddr}
	tid := trace.Ensure("")
	a.runConsoleChat(w, r, p, req, tid)
}

func (a *App) runConsoleChat(w http.ResponseWriter, r *http.Request, p *auth.Principal, req channel.ChatRequest, tid string) {
	// 复用 chat 管道：构造最小内部调用
	if req.Model == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "model required"})
		return
	}
	model, err := a.Store.GetModelByName(req.Model)
	if err != nil || !model.Enabled {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "model not found"})
		return
	}
	ok, _ := a.Store.GroupAllowsModel(p.GroupID, model.ID)
	if !ok {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "model not allowed"})
		return
	}
	maxTokens := model.MaxTokens
	if req.MaxTokens != nil && *req.MaxTokens > 0 {
		maxTokens = *req.MaxTokens
	}
	frozen := a.Billing.Estimate(model, p.Group.Multiplier, maxTokens)
	if err := a.Billing.Freeze(p.User.ID, p.Key.ID, frozen, tid); err != nil {
		writeJSON(w, http.StatusPaymentRequired, map[string]any{"error": err.Error()})
		return
	}
	candidates, err := a.Router.SelectAllOrdered(model.ID)
	if err != nil || len(candidates) == 0 {
		_ = a.Billing.Unfreeze(p.User.ID, p.Key.ID, frozen, tid)
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": "no channel"})
		return
	}
	var lastErr error
	for _, mc := range candidates {
		adapter, err := a.Adapters.Get(mc.Channel.Provider)
		if err != nil {
			lastErr = err
			continue
		}
		resp, err := adapter.Chat(r.Context(), mc.Channel, mc.UpstreamModel, req)
		if err != nil {
			lastErr = err
			continue
		}
		inTok, outTok := resp.Usage.PromptTokens, resp.Usage.CompletionTokens
		if inTok == 0 {
			inTok = channel.EstimatePromptTokens(req.Messages)
		}
		cost := a.Billing.Actual(model, p.Group.Multiplier, inTok, outTok)
		_ = a.Billing.Settle(p.User.ID, p.Key.ID, frozen, cost, tid)
		a.Store.AddQuotaUsed(p.Key.ID, int64(inTok+outTok))
		a.logReq(p, model.ModelName, mc.Channel.ID, inTok, outTok, cost, p.Group.Multiplier, http.StatusOK, true, 0, tid, "")
		resp.Model = model.ModelName
		writeJSON(w, http.StatusOK, resp)
		return
	}
	_ = a.Billing.Unfreeze(p.User.ID, p.Key.ID, frozen, tid)
	msg := "upstream failed"
	if lastErr != nil {
		msg = lastErr.Error()
	}
	writeJSON(w, http.StatusBadGateway, map[string]any{"error": msg})
}

func (a *App) handleAdminOverview(w http.ResponseWriter, r *http.Request) {
	claims, err := a.requireJWT(r)
	if err != nil || !claims.IsAdmin {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "admin only"})
		return
	}
	stats, err := a.Store.StatsOverview()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (a *App) handleAdminChannelsJWT(w http.ResponseWriter, r *http.Request) {
	claims, err := a.requireJWT(r)
	if err != nil || !claims.IsAdmin {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "admin only"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		list, err := a.Store.ListChannels()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		items := make([]map[string]any, 0, len(list))
		for _, c := range list {
			items = append(items, map[string]any{
				"id": c.ID, "name": c.Name, "provider": c.Provider, "base_url": c.BaseURL,
				"api_key_masked": store.MaskSecret(c.APIKeyEnc), "weight": c.Weight, "priority": c.Priority,
				"status": c.Status, "timeout_ms": c.TimeoutMs,
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	case http.MethodPost:
		// 复用原 admin token handler 逻辑
		r.Header.Set("X-Admin-Token", a.Cfg.Auth.AdminPassword)
		a.handleAdminChannels(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "GET/POST only"})
	}
}

func (a *App) handleAdminModelsJWT(w http.ResponseWriter, r *http.Request) {
	claims, err := a.requireJWT(r)
	if err != nil || !claims.IsAdmin {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "admin only"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		list, err := a.Store.ListAllModels()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": list})
	case http.MethodPost:
		r.Header.Set("X-Admin-Token", a.Cfg.Auth.AdminPassword)
		a.handleAdminModels(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "GET/POST only"})
	}
}
