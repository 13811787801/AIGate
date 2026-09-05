package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"aigate/internal/auth"
	"aigate/internal/billing"
	"aigate/internal/channel"
	"aigate/internal/config"
	"aigate/internal/ratelimit"
	"aigate/internal/router"
	"aigate/internal/store"
	"aigate/pkg/trace"
)

type App struct {
	Cfg      *config.Config
	Store    *store.Store
	Auth     *auth.Service
	Billing  *billing.Service
	Router   *router.Service
	Adapters *channel.Registry
	Limit    *ratelimit.Limiter
}

func Run(cfg *config.Config) error {
	st, err := store.Open(cfg.Database.Driver, cfg.Database.SQLitePath, cfg.Database.DSN)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer st.Close()

	seed, err := st.SeedIfEmpty(cfg.Auth.AdminUsername, cfg.Auth.AdminPassword)
	if err != nil {
		return fmt.Errorf("seed: %w", err)
	}
	if seed.Created {
		log.Printf("========== DEMO API KEY (save it, shown once) ==========")
		log.Printf("%s", seed.DemoAPIKey)
		log.Printf("========================================================")
	}

	app := &App{
		Cfg:   cfg,
		Store: st,
		Auth:  &auth.Service{Store: st},
		Billing: &billing.Service{
			Store:        st,
			FreezeMargin: cfg.Billing.FreezeMargin,
		},
		Router:   &router.Service{Store: st},
		Adapters: channel.NewRegistry(channel.MockAdapter{}, channel.NewOpenAICompatible()),
		Limit:    ratelimit.New(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", app.handleHealth)
	mux.HandleFunc("/v1/models", app.handleModels)
	mux.HandleFunc("/v1/chat/completions", app.handleChat)

	// Phase1 bootstrap (header token)
	mux.HandleFunc("/api/admin/channels", app.handleAdminChannels)
	mux.HandleFunc("/api/admin/models", app.handleAdminModels)
	mux.HandleFunc("/api/admin/wallet", app.handleAdminWallet)

	// Phase2 console API (JWT)
	mux.HandleFunc("/api/auth/login", app.handleLogin)
	mux.HandleFunc("/api/me", app.handleMe)
	mux.HandleFunc("/api/wallet", app.handleConsoleWallet)
	mux.HandleFunc("/api/keys", app.handleConsoleKeys)
	mux.HandleFunc("/api/logs", app.handleConsoleLogs)
	mux.HandleFunc("/api/models", app.handleConsoleModels)
	mux.HandleFunc("/api/chat", app.handleConsoleChat)
	mux.HandleFunc("/api/admin/overview", app.handleAdminOverview)
	mux.HandleFunc("/api/admin/console/channels", app.handleAdminChannelsJWT)
	mux.HandleFunc("/api/admin/console/models", app.handleAdminModelsJWT)

	mux.Handle("/", consoleFileServer())

	addr := cfg.Addr()
	log.Printf("AIGate listening on http://%s", addr)
	log.Printf("Console UI: http://127.0.0.1:%d/", cfg.Server.Port)
	log.Printf("Health:     http://127.0.0.1:%d/healthz", cfg.Server.Port)
	return http.ListenAndServe(addr, withAccessLog(mux))
}

func (a *App) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"service": "aigate",
		"phase":   2,
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

func (a *App) handleModels(w http.ResponseWriter, r *http.Request) {
	tid := trace.Ensure(r.Header.Get("X-Trace-Id"))
	w.Header().Set("X-Trace-Id", tid)
	p, err := a.Auth.Authenticate(r)
	if err != nil {
		writeAuthError(w, tid, err)
		return
	}
	models, err := a.Store.ListEnabledModelsForGroup(p.GroupID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, tid, "server_error", err.Error())
		return
	}
	data := make([]map[string]any, 0, len(models))
	now := time.Now().Unix()
	for _, m := range models {
		data = append(data, map[string]any{
			"id": m.ModelName, "object": "model", "created": now, "owned_by": "aigate",
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": data})
}

func (a *App) handleChat(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	tid := trace.Ensure(r.Header.Get("X-Trace-Id"))
	w.Header().Set("X-Trace-Id", tid)

	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, tid, "method_not_allowed", "POST required")
		return
	}
	p, err := a.Auth.Authenticate(r)
	if err != nil {
		writeAuthError(w, tid, err)
		return
	}
	if !a.Limit.AllowRPM(p.Key.ID, p.Key.RPMLimit) {
		writeError(w, http.StatusTooManyRequests, tid, "rate_limit_exceeded", "rpm limit exceeded")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, tid, "invalid_request", "read body failed")
		return
	}
	var req channel.ChatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, tid, "invalid_request", "invalid json")
		return
	}
	if req.Model == "" {
		writeError(w, http.StatusBadRequest, tid, "invalid_request", "model is required")
		return
	}
	if !p.AllowModel(req.Model) {
		writeError(w, http.StatusForbidden, tid, "model_not_allowed", auth.ErrModelDenied.Error())
		return
	}

	model, err := a.Store.GetModelByName(req.Model)
	if err != nil || !model.Enabled {
		writeError(w, http.StatusNotFound, tid, "model_not_found", "model not found or disabled")
		return
	}
	ok, err := a.Store.GroupAllowsModel(p.GroupID, model.ID)
	if err != nil || !ok {
		writeError(w, http.StatusForbidden, tid, "model_not_allowed", "model not in your group")
		return
	}

	maxTokens := model.MaxTokens
	if req.MaxTokens != nil && *req.MaxTokens > 0 {
		maxTokens = *req.MaxTokens
	}
	frozen := a.Billing.Estimate(model, p.Group.Multiplier, maxTokens)
	if err := a.Billing.Freeze(p.User.ID, p.Key.ID, frozen, tid); err != nil {
		var ib store.InsufficientBalanceError
		if errors.As(err, &ib) {
			writeError(w, http.StatusPaymentRequired, tid, "insufficient_quota",
				fmt.Sprintf("insufficient balance: have=%.6f need=%.6f", ib.Balance, ib.Need))
			return
		}
		writeError(w, http.StatusInternalServerError, tid, "billing_error", err.Error())
		return
	}

	candidates, err := a.Router.SelectAllOrdered(model.ID)
	if err != nil || len(candidates) == 0 {
		_ = a.Billing.Unfreeze(p.User.ID, p.Key.ID, frozen, tid)
		writeError(w, http.StatusBadGateway, tid, "no_channel", "no upstream channel")
		return
	}

	ctx := r.Context()
	var lastErr error
	for _, mc := range candidates {
		adapter, err := a.Adapters.Get(mc.Channel.Provider)
		if err != nil {
			lastErr = err
			continue
		}
		if req.Stream {
			err = a.doStream(ctx, w, r, p, model, mc, adapter, req, frozen, tid, start)
		} else {
			err = a.doNonStream(ctx, w, r, p, model, mc, adapter, req, frozen, tid, start)
		}
		if err == nil {
			return
		}
		lastErr = err
		log.Printf("channel %s failed: %v", mc.Channel.Name, err)
	}
	_ = a.Billing.Unfreeze(p.User.ID, p.Key.ID, frozen, tid)
	msg := "all channels failed"
	if lastErr != nil {
		msg = lastErr.Error()
	}
	a.logReq(p, model.ModelName, 0, 0, 0, 0, p.Group.Multiplier, http.StatusBadGateway, false, int(time.Since(start).Milliseconds()), tid, msg)
	writeError(w, http.StatusBadGateway, tid, "upstream_error", msg)
}

func (a *App) doNonStream(ctx context.Context, w http.ResponseWriter, r *http.Request, p *auth.Principal,
	model *store.ModelConfig, mc store.ModelChannel, adapter channel.Adapter, req channel.ChatRequest,
	frozen float64, tid string, start time.Time) error {

	resp, err := adapter.Chat(ctx, mc.Channel, mc.UpstreamModel, req)
	if err != nil {
		return err
	}
	inTok, outTok := resp.Usage.PromptTokens, resp.Usage.CompletionTokens
	if inTok == 0 {
		inTok = channel.EstimatePromptTokens(req.Messages)
	}
	cost := a.Billing.Actual(model, p.Group.Multiplier, inTok, outTok)
	if err := a.Billing.Settle(p.User.ID, p.Key.ID, frozen, cost, tid); err != nil {
		log.Printf("settle error: %v", err)
	}
	a.Store.AddQuotaUsed(p.Key.ID, int64(inTok+outTok))
	a.logReq(p, model.ModelName, mc.Channel.ID, inTok, outTok, cost, p.Group.Multiplier, http.StatusOK, true, int(time.Since(start).Milliseconds()), tid, "")

	// 对外 model 名用客户端请求的名字
	resp.Model = model.ModelName
	writeJSON(w, http.StatusOK, resp)
	return nil
}

func (a *App) doStream(ctx context.Context, w http.ResponseWriter, r *http.Request, p *auth.Principal,
	model *store.ModelConfig, mc store.ModelChannel, adapter channel.Adapter, req channel.ChatRequest,
	frozen float64, tid string, start time.Time) error {

	ch, err := adapter.ChatStream(ctx, mc.Channel, mc.UpstreamModel, req)
	if err != nil {
		return err
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming not supported by server")
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	var completion strings.Builder
	var usage channel.Usage
	for chunk := range ch {
		if chunk.Err != nil {
			_ = a.Billing.Unfreeze(p.User.ID, p.Key.ID, frozen, tid)
			return chunk.Err
		}
		if chunk.Done {
			break
		}
		// 尝试抽取 usage / content
		var partial map[string]any
		if json.Unmarshal(chunk.Data, &partial) == nil {
			if u, ok := partial["usage"].(map[string]any); ok {
				usage.PromptTokens = asInt(u["prompt_tokens"])
				usage.CompletionTokens = asInt(u["completion_tokens"])
			}
			if choices, ok := partial["choices"].([]any); ok && len(choices) > 0 {
				if c0, ok := choices[0].(map[string]any); ok {
					if delta, ok := c0["delta"].(map[string]any); ok {
						if content, ok := delta["content"].(string); ok {
							completion.WriteString(content)
						}
					}
				}
			}
			partial["model"] = model.ModelName
			chunk.Data, _ = json.Marshal(partial)
		}
		fmt.Fprintf(w, "data: %s\n\n", chunk.Data)
		flusher.Flush()
	}
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()

	inTok := usage.PromptTokens
	outTok := usage.CompletionTokens
	if inTok == 0 {
		inTok = channel.EstimatePromptTokens(req.Messages)
	}
	if outTok == 0 {
		outTok = len([]rune(completion.String())) / 2
		if outTok < 1 && completion.Len() > 0 {
			outTok = 1
		}
	}
	cost := a.Billing.Actual(model, p.Group.Multiplier, inTok, outTok)
	if err := a.Billing.Settle(p.User.ID, p.Key.ID, frozen, cost, tid); err != nil {
		log.Printf("settle error: %v", err)
	}
	a.Store.AddQuotaUsed(p.Key.ID, int64(inTok+outTok))
	a.logReq(p, model.ModelName, mc.Channel.ID, inTok, outTok, cost, p.Group.Multiplier, http.StatusOK, true, int(time.Since(start).Milliseconds()), tid, "")
	return nil
}

func (a *App) logReq(p *auth.Principal, model string, channelID int64, in, out int, cost, mult float64, code int, ok bool, latency int, tid, errMsg string) {
	var chID, gID sql.NullInt64
	if channelID > 0 {
		chID = sql.NullInt64{Int64: channelID, Valid: true}
	}
	gID = sql.NullInt64{Int64: p.GroupID, Valid: true}
	_ = a.Store.InsertRequestLog(store.RequestLog{
		TraceID: tid, UserID: p.User.ID, APIKeyID: p.Key.ID, ModelName: model, ChannelID: chID, GroupID: gID,
		InputTokens: in, OutputTokens: out, Cost: cost, Multiplier: mult, StatusCode: code, Success: ok,
		LatencyMs: latency, IP: p.ClientIP, ErrorMessage: errMsg,
	})
}

// --- admin bootstrap (Phase1: X-Admin-Token = admin password) ---

func (a *App) checkAdmin(r *http.Request) bool {
	return r.Header.Get("X-Admin-Token") == a.Cfg.Auth.AdminPassword
}

func (a *App) handleAdminChannels(w http.ResponseWriter, r *http.Request) {
	if !a.checkAdmin(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST only"})
		return
	}
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
	if body.Provider == "" {
		body.Provider = "openai_compatible"
	}
	if body.Weight == 0 {
		body.Weight = 100
	}
	if body.TimeoutMs == 0 {
		body.TimeoutMs = 120000
	}
	id, err := a.Store.CreateChannel(store.Channel{
		Name: body.Name, Provider: body.Provider, BaseURL: body.BaseURL, APIKeyEnc: body.APIKey,
		Weight: body.Weight, Priority: body.Priority, Status: 1, TimeoutMs: body.TimeoutMs,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id})
}

func (a *App) handleAdminModels(w http.ResponseWriter, r *http.Request) {
	if !a.checkAdmin(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST only"})
		return
	}
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
	if body.BillingMode == "" {
		body.BillingMode = "TOKEN"
	}
	if body.MaxTokens == 0 {
		body.MaxTokens = 8192
	}
	if body.UpstreamModel == "" {
		body.UpstreamModel = body.ModelName
	}
	if body.GroupID == 0 {
		body.GroupID = 1
	}
	id, err := a.Store.CreateModel(store.ModelConfig{
		ModelName: body.ModelName, DisplayName: body.DisplayName, BillingMode: body.BillingMode,
		InputPricePer1K: body.InputPricePer1K, OutputPricePer1K: body.OutputPricePer1K,
		MaxTokens: body.MaxTokens, SupportsStream: true, Enabled: true,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if body.ChannelID > 0 {
		if err := a.Store.BindModelChannel(id, body.ChannelID, body.UpstreamModel, 100); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
	}
	_ = a.Store.BindGroupModel(body.GroupID, id)
	writeJSON(w, http.StatusOK, map[string]any{"id": id})
}

func (a *App) handleAdminWallet(w http.ResponseWriter, r *http.Request) {
	if !a.checkAdmin(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	u, err := a.Store.GetUser(1)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"user_id": u.ID, "username": u.Username, "balance": u.Balance, "frozen": u.Frozen, "total_consumed": u.TotalConsumed,
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, tid, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]any{
			"message": message,
			"type":    "aigate_error",
			"param":   nil,
			"code":    code,
		},
		"trace_id": tid,
	})
}

func writeAuthError(w http.ResponseWriter, tid string, err error) {
	switch {
	case errors.Is(err, auth.ErrMissingKey), errors.Is(err, auth.ErrInvalidKey):
		writeError(w, http.StatusUnauthorized, tid, "invalid_api_key", err.Error())
	case errors.Is(err, auth.ErrKeyDisabled), errors.Is(err, auth.ErrKeyExpired), errors.Is(err, auth.ErrUserDisabled):
		writeError(w, http.StatusForbidden, tid, "forbidden", err.Error())
	case errors.Is(err, auth.ErrIPDenied):
		writeError(w, http.StatusForbidden, tid, "ip_not_allowed", err.Error())
	default:
		writeError(w, http.StatusInternalServerError, tid, "auth_error", err.Error())
	}
}

func withAccessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

func asInt(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	default:
		return 0
	}
}
