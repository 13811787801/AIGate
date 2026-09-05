package server

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"aigate/internal/config"
	"aigate/pkg/trace"
)

// Run 启动 HTTP 服务（Phase 0：健康检查 + 占位 OpenAI 兼容路由）
func Run(cfg *config.Config) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "ok",
			"service": "aigate",
			"time":    time.Now().UTC().Format(time.RFC3339),
		})
	})

	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		tid := trace.Ensure(r.Header.Get("X-Trace-Id"))
		w.Header().Set("X-Trace-Id", tid)
		// Phase 0 占位：后续接鉴权 + 模型表
		writeJSON(w, http.StatusOK, map[string]any{
			"object": "list",
			"data":   []any{},
			"hint":   "phase0 scaffold — wire store in phase1",
		})
	})

	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		tid := trace.Ensure(r.Header.Get("X-Trace-Id"))
		w.Header().Set("X-Trace-Id", tid)
		writeJSON(w, http.StatusNotImplemented, map[string]any{
			"error": map[string]any{
				"message": "not implemented in phase0 scaffold; coming in phase1",
				"type":    "aigate_error",
				"code":    "not_implemented",
			},
			"trace_id": tid,
		})
	})

	addr := cfg.Addr()
	log.Printf("AIGate listening on http://%s (mode=%s, plugin=%v)", addr, cfg.Server.Mode, cfg.Plugin.Enabled)
	log.Printf("health: http://127.0.0.1:%d/healthz", cfg.Server.Port)
	return http.ListenAndServe(addr, withAccessLog(mux))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func withAccessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
