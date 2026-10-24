package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"aigate/internal/auth"
	"aigate/internal/authtoken"
	"aigate/internal/channel"
	"aigate/internal/crypto"
	"aigate/internal/modules"
	"aigate/internal/store"
	"aigate/pkg/trace"
)

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
	if a.Lic.Loaded && !a.Lic.Valid {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "instance license invalid", "detail": a.Lic.Reason})
		return
	}
	if a.isSaasMode() && a.Cfg.License.Require && (!a.Lic.Loaded || !a.Lic.Valid) {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "saas license required"})
		return
	}
	isAdmin := u.Role == "admin" || a.isAdminUser(u.Username)
	token, err := authtoken.Issue(a.Cfg.Auth.JWTSecret, u.ID, u.Username, isAdmin, 0)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	snap := a.Ent.ForUser(u, isAdmin)
	a.Store.WriteAudit(u.ID, "login", u.Username, "", clientIPFrom(r))
	writeJSON(w, http.StatusOK, map[string]any{
		"token": token,
		"user": map[string]any{
			"id": u.ID, "username": u.Username, "display_name": u.DisplayName,
			"is_admin": isAdmin, "role": u.Role, "balance": u.Balance, "frozen": u.Frozen, "total_consumed": u.TotalConsumed,
			"plan_id": u.PlanID, "parent_id": u.ParentID, "org_role": u.OrgRole,
		},
		"entitlements": snap,
		"modules":      snap.Modules,
	})
}

func (a *App) handleModules(w http.ResponseWriter, r *http.Request) {
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

func (a *App) handleMe(w http.ResponseWriter, r *http.Request) {
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
	writeJSON(w, http.StatusOK, out["user"])
}

func (a *App) handleConsoleWallet(w http.ResponseWriter, r *http.Request) {
	claims, err := a.requireJWT(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	if !a.requireModuleFor(w, r, claims, modules.Wallet) {
		return
	}
	u, err := a.Store.GetUser(claims.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"balance": u.Balance, "frozen": u.Frozen, "total_consumed": u.TotalConsumed,
		"demo_codes": []string{"AIGATE-WELCOME", "AIGATE-DEMO-100"},
	})
}

func (a *App) handleWalletRedeem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST only"})
		return
	}
	claims, err := a.requireJWT(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	if !a.requireModuleFor(w, r, claims, modules.Wallet) {
		return
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad json"})
		return
	}
	amount, balance, err := a.Store.RedeemCode(claims.UserID, body.Code)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrRedeemInvalid):
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "兑换码无效或已停用"})
		case errors.Is(err, store.ErrRedeemUsed):
			writeJSON(w, http.StatusConflict, map[string]any{"error": "该兑换码你已使用过"})
		case errors.Is(err, store.ErrRedeemExhausted):
			writeJSON(w, http.StatusConflict, map[string]any{"error": "兑换码已兑完"})
		default:
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "amount": amount, "balance": balance})
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
			expired := ""
			if k.ExpiredAt.Valid {
				expired = k.ExpiredAt.String
			}
			items = append(items, map[string]any{
				"id": k.ID, "name": k.Name, "key_prefix": k.KeyPrefix + "...",
				"status": k.Status, "rpm_limit": k.RPMLimit, "tpm_limit": k.TPMLimit,
				"quota_used": k.QuotaUsed, "allowed_models": k.AllowedModels, "ip_whitelist": k.IPWhitelist,
				"expired_at": expired,
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	case http.MethodPost:
		var body struct {
			Name          string   `json:"name"`
			RPM           int      `json:"rpm_limit"`
			TPM           int      `json:"tpm_limit"`
			ExpiredAt     string   `json:"expired_at"`
			AllowedModels []string `json:"allowed_models"`
			IPWhitelist   []string `json:"ip_whitelist"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		plain, key, err := a.Store.CreateUserAPIKeyOpts(claims.UserID, store.APIKeyCreateOpts{
			Name: body.Name, RPM: body.RPM, TPM: body.TPM,
			ExpiredAt: body.ExpiredAt, AllowedModels: body.AllowedModels, IPWhitelist: body.IPWhitelist,
		})
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"id": key.ID, "name": key.Name, "api_key": plain,
			"hint": "请立即保存，系统只存储哈希，无法再次查看完整密钥",
		})
	case http.MethodPatch:
		var body struct {
			ID     int64 `json:"id"`
			Status *int  `json:"status"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ID <= 0 || body.Status == nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "id and status required"})
			return
		}
		if *body.Status != 0 && *body.Status != 1 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "status must be 0 or 1"})
			return
		}
		if err := a.Store.SetAPIKeyStatus(claims.UserID, body.ID, *body.Status); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
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
	if !a.requireModuleFor(w, r, claims, modules.Playground) {
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad body"})
		return
	}
	var incoming struct {
		channel.ChatRequest
		ConversationID int64  `json:"conversation_id"`
		Content        string `json:"content"`
	}
	if err := json.Unmarshal(body, &incoming); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad json"})
		return
	}
	req := incoming.ChatRequest
	req.Stream = false
	userText := strings.TrimSpace(incoming.Content)
	sessionID := incoming.ConversationID
	if userText != "" {
		msgs := make([]channel.ChatMessage, 0, 8)
		if sessionID > 0 {
			if _, err = a.Store.GetChatSession(claims.UserID, sessionID); err != nil {
				writeJSON(w, http.StatusNotFound, map[string]any{"error": "conversation not found"})
				return
			}
			history, err := a.Store.ListChatMessages(claims.UserID, sessionID)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
				return
			}
			for _, m := range history {
				msgs = append(msgs, channel.ChatMessage{Role: m.Role, Content: m.Content})
			}
		}
		msgs = append(msgs, channel.ChatMessage{Role: "user", Content: userText})
		if len(msgs) > 40 {
			msgs = msgs[len(msgs)-40:]
		}
		req.Messages = msgs
	} else if len(req.Messages) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "content required"})
		return
	} else {
		sessionID = 0
	}

	keys, err := a.Store.ListAPIKeysByUser(claims.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if len(keys) == 0 {
		_, key, err := a.Store.CreateUserAPIKey(claims.UserID, "playground-auto", 60, 100000)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "请先创建至少一个 API 令牌", "detail": err.Error()})
			return
		}
		keys = []store.APIKey{*key}
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
	a.runConsoleChat(w, r, p, req, tid, sessionID, userText)
}

func (a *App) runConsoleChat(w http.ResponseWriter, r *http.Request, p *auth.Principal, req channel.ChatRequest, tid string, sessionID int64, userText string) {
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
		mc, err := a.unlockModelChannel(mc)
		if err != nil {
			lastErr = err
			continue
		}
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
		if userText != "" {
			answer := ""
			if len(resp.Choices) > 0 {
				answer = resp.Choices[0].Message.Content
			}
			if sessionID == 0 {
				sessionID, err = a.Store.CreateChatSession(p.User.ID, userText, model.ModelName)
				if err != nil {
					writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
					return
				}
			}
			if err := a.Store.AppendChatTurn(p.User.ID, sessionID, userText, answer, model.ModelName); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "回答已生成，但保存会话失败: " + err.Error(), "conversation_id": sessionID})
				return
			}
		}
		writeJSON(w, http.StatusOK, struct {
			*channel.ChatResponse
			ConversationID int64 `json:"conversation_id"`
		}{ChatResponse: resp, ConversationID: sessionID})
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
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	if !a.requireModuleFor(w, r, claims, modules.AdminChannels) {
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
			plain, _ := crypto.Open(a.DataKey, c.APIKeyEnc)
			items = append(items, map[string]any{
				"id": c.ID, "name": c.Name, "provider": c.Provider, "base_url": c.BaseURL,
				"api_key_masked": store.MaskSecret(plain), "weight": c.Weight, "priority": c.Priority,
				"status": c.Status, "timeout_ms": c.TimeoutMs,
				"key_encrypted": crypto.IsSealed(c.APIKeyEnc),
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	case http.MethodPost:
		var body struct {
			Name      string `json:"name"`
			Provider  string `json:"provider"`
			BaseURL   string `json:"base_url"`
			APIKey    string `json:"api_key"`
			Weight    int    `json:"weight"`
			Priority  int    `json:"priority"`
			TimeoutMs int    `json:"timeout_ms"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad json"})
			return
		}
		id, err := a.createChannelFromBody(body.Name, body.Provider, body.BaseURL, body.APIKey, body.Weight, body.Priority, body.TimeoutMs)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"id": id})
	case http.MethodPatch:
		var body struct {
			ID       int64   `json:"id"`
			Status   *int    `json:"status"`
			Name     *string `json:"name"`
			Provider *string `json:"provider"`
			BaseURL  *string `json:"base_url"`
			APIKey   *string `json:"api_key"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ID <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "id required"})
			return
		}
		if body.Name != nil || body.Provider != nil || body.BaseURL != nil || (body.APIKey != nil && strings.TrimSpace(*body.APIKey) != "") {
			cur, err := a.Store.GetChannel(body.ID)
			if err != nil {
				if errors.Is(err, store.ErrNotFound) {
					writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
					return
				}
				writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
				return
			}
			name, provider, baseURL := cur.Name, cur.Provider, cur.BaseURL
			if body.Name != nil {
				name = strings.TrimSpace(*body.Name)
			}
			if body.Provider != nil && strings.TrimSpace(*body.Provider) != "" {
				provider = strings.TrimSpace(*body.Provider)
			}
			if body.BaseURL != nil {
				baseURL = strings.TrimSpace(*body.BaseURL)
			}
			if provider != "mock" && baseURL == "" {
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": "base_url required"})
				return
			}
			replaceKey := false
			sealed := ""
			if body.APIKey != nil && strings.TrimSpace(*body.APIKey) != "" {
				sealed, err = a.sealAPIKey(strings.TrimSpace(*body.APIKey))
				if err != nil {
					writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
					return
				}
				replaceKey = true
			}
			if err := a.Store.UpdateChannel(body.ID, name, provider, baseURL, sealed, replaceKey); err != nil {
				if errors.Is(err, store.ErrNotFound) {
					writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
					return
				}
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"ok": true})
			return
		}
		if body.Status == nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "id and status required"})
			return
		}
		if err := a.Store.SetChannelStatus(body.ID, *body.Status); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "GET/POST/PATCH only"})
	}
}

func (a *App) handleAdminChannelTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST only"})
		return
	}
	claims, err := a.requireJWT(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	if !a.requireModuleFor(w, r, claims, modules.AdminChannels) {
		return
	}
	var body struct {
		ID            int64  `json:"id"`
		UpstreamModel string `json:"upstream_model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "id required"})
		return
	}
	ch, err := a.Store.GetChannel(body.ID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "channel not found"})
		return
	}
	unlocked, err := a.openChannel(*ch)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "decrypt channel key: " + err.Error()})
		return
	}
	adapter, err := a.Adapters.Get(unlocked.Provider)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	model := strings.TrimSpace(body.UpstreamModel)
	if model == "" {
		if lister, ok := adapter.(interface {
			ListModelIDs(ctx context.Context, ch store.Channel) ([]string, error)
		}); ok {
			ids, listErr := lister.ListModelIDs(r.Context(), unlocked)
			if listErr != nil && !probeListSkippable(listErr) {
				writeJSON(w, http.StatusBadGateway, map[string]any{"ok": false, "error": listErr.Error()})
				return
			}
			model = channel.PickProbeModel(unlocked.Provider, ids)
		} else {
			model = channel.PickProbeModel(unlocked.Provider, nil)
		}
	}
	ctx := r.Context()
	resp, err := adapter.Chat(ctx, unlocked, model, channel.ChatRequest{
		Model:    model,
		Messages: []channel.ChatMessage{{Role: "user", Content: "ping"}},
	})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	preview := ""
	if len(resp.Choices) > 0 {
		preview = resp.Choices[0].Message.Content
		if len(preview) > 120 {
			preview = preview[:120] + "…"
		}
	}
	used := resp.Model
	if used == "" {
		used = model
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "preview": preview, "model": used})
}

// probeListSkippable is true when GET /models is missing or unreachable.
// Auth failures stay visible so a bad key is not hidden behind a guessed model name.
func probeListSkippable(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return !strings.Contains(msg, "upstream 401") && !strings.Contains(msg, "upstream 403")
}

func (a *App) handleAdminModelsJWT(w http.ResponseWriter, r *http.Request) {
	claims, err := a.requireJWT(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	if !a.requireModuleFor(w, r, claims, modules.AdminChannels) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		list, err := a.Store.ListAllModels()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		bindings, err := a.Store.ListModelBindingSummary()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		items := make([]map[string]any, 0, len(list))
		for _, m := range list {
			b := bindings[m.ID]
			items = append(items, map[string]any{
				"ID": m.ID, "id": m.ID,
				"ModelName": m.ModelName, "model_name": m.ModelName,
				"DisplayName": m.DisplayName, "display_name": m.DisplayName,
				"BillingMode": m.BillingMode, "billing_mode": m.BillingMode,
				"InputPricePer1K": m.InputPricePer1K, "input_price_per_1k": m.InputPricePer1K,
				"OutputPricePer1K": m.OutputPricePer1K, "output_price_per_1k": m.OutputPricePer1K,
				"MaxTokens": m.MaxTokens, "max_tokens": m.MaxTokens,
				"SupportsStream": m.SupportsStream, "supports_stream": m.SupportsStream,
				"Enabled": m.Enabled, "enabled": m.Enabled,
				"channel_id": b.ChannelID, "upstream_model": b.Upstream, "channel_name": b.ChannelName,
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	case http.MethodPost:
		var body struct {
			ModelName        string  `json:"model_name"`
			DisplayName      string  `json:"display_name"`
			BillingMode      string  `json:"billing_mode"`
			InputPricePer1K  float64 `json:"input_price_per_1k"`
			OutputPricePer1K float64 `json:"output_price_per_1k"`
			MaxTokens        int     `json:"max_tokens"`
			ChannelID        int64   `json:"channel_id"`
			UpstreamModel    string  `json:"upstream_model"`
			GroupID          int64   `json:"group_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad json"})
			return
		}
		id, err := a.createModelFromBody(body.ModelName, body.DisplayName, body.BillingMode, body.InputPricePer1K, body.OutputPricePer1K, body.MaxTokens, body.ChannelID, body.UpstreamModel, body.GroupID)
		if err != nil {
			code := http.StatusInternalServerError
			if err.Error() == "model_name required" {
				code = http.StatusBadRequest
			}
			writeJSON(w, code, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"id": id})
	case http.MethodPatch:
		var body struct {
			ID               int64    `json:"id"`
			Enabled          *bool    `json:"enabled"`
			ModelName        *string  `json:"model_name"`
			DisplayName      *string  `json:"display_name"`
			BillingMode      string   `json:"billing_mode"`
			InputPricePer1K  *float64 `json:"input_price_per_1k"`
			OutputPricePer1K *float64 `json:"output_price_per_1k"`
			MaxTokens        int      `json:"max_tokens"`
			ChannelID        int64    `json:"channel_id"`
			UpstreamModel    *string  `json:"upstream_model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ID <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "id required"})
			return
		}
		if body.ModelName != nil || body.UpstreamModel != nil || body.ChannelID > 0 || body.DisplayName != nil || body.InputPricePer1K != nil || body.OutputPricePer1K != nil {
			cur, err := a.Store.GetModel(body.ID)
			if err != nil {
				if errors.Is(err, store.ErrNotFound) {
					writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
					return
				}
				writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
				return
			}
			next := *cur
			if body.ModelName != nil {
				next.ModelName = strings.TrimSpace(*body.ModelName)
			}
			if body.DisplayName != nil {
				next.DisplayName = strings.TrimSpace(*body.DisplayName)
			}
			if body.BillingMode != "" {
				next.BillingMode = body.BillingMode
			}
			if body.InputPricePer1K != nil {
				next.InputPricePer1K = *body.InputPricePer1K
			}
			if body.OutputPricePer1K != nil {
				next.OutputPricePer1K = *body.OutputPricePer1K
			}
			if body.MaxTokens > 0 {
				next.MaxTokens = body.MaxTokens
			}
			if err := a.Store.UpdateModel(next); err != nil {
				writeModelWriteError(w, err)
				return
			}
			if body.ChannelID > 0 || body.UpstreamModel != nil {
				upstream := ""
				if body.UpstreamModel != nil {
					upstream = strings.TrimSpace(*body.UpstreamModel)
				}
				channelID := body.ChannelID
				if channelID <= 0 || upstream == "" {
					bindings, berr := a.Store.ListModelBindingSummary()
					if berr != nil {
						writeJSON(w, http.StatusInternalServerError, map[string]any{"error": berr.Error()})
						return
					}
					if channelID <= 0 {
						channelID = bindings[body.ID].ChannelID
					}
					if upstream == "" {
						upstream = bindings[body.ID].Upstream
					}
				}
				if upstream == "" {
					upstream = next.ModelName
				}
				if channelID <= 0 {
					writeJSON(w, http.StatusBadRequest, map[string]any{"error": "channel_id required"})
					return
				}
				if err := a.Store.SetModelPrimaryChannel(body.ID, channelID, upstream); err != nil {
					writeModelWriteError(w, err)
					return
				}
			}
			writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": body.ID})
			return
		}
		if body.Enabled == nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "id and enabled required"})
			return
		}
		if err := a.Store.SetModelEnabled(body.ID, *body.Enabled); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "GET/POST/PATCH only"})
	}
}

func writeModelWriteError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
	case errors.Is(err, store.ErrDuplicate):
		writeJSON(w, http.StatusConflict, map[string]any{"error": "对外模型名已存在，请直接编辑那一条"})
	case err.Error() == "model_name required" || err.Error() == "upstream_model required" || err.Error() == "channel_id required":
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
}
