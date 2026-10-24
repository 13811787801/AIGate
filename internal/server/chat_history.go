package server

import (
	"net/http"
	"strconv"

	"aigate/internal/modules"
	"aigate/internal/store"
)

func (a *App) handleChatSessions(w http.ResponseWriter, r *http.Request) {
	claims, err := a.requireJWT(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	if !a.requireModuleFor(w, r, claims, modules.Playground) {
		return
	}
	id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	switch r.Method {
	case http.MethodGet:
		if id <= 0 {
			list, err := a.Store.ListChatSessions(claims.UserID, 50)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
				return
			}
			if list == nil {
				list = []store.ChatSession{}
			}
			writeJSON(w, http.StatusOK, map[string]any{"items": list})
			return
		}
		sess, err := a.Store.GetChatSession(claims.UserID, id)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "conversation not found"})
			return
		}
		msgs, err := a.Store.ListChatMessages(claims.UserID, id)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		if msgs == nil {
			msgs = []store.ChatTurn{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"conversation": sess, "messages": msgs})
	case http.MethodDelete:
		if id <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "id required"})
			return
		}
		if err := a.Store.DeleteChatSession(claims.UserID, id); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "conversation not found"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "GET/DELETE only"})
	}
}
