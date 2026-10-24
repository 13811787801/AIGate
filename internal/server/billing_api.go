package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"aigate/internal/store"
)

// handleBillingCatalog: SaaS plan price list (mock). Available to any logged-in user in saas mode.
func (a *App) handleBillingCatalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	if _, err := a.requireJWT(r); err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	if !a.isSaasMode() {
		writeJSON(w, http.StatusForbidden, map[string]any{
			"error": "billing catalog only in saas mode",
			"hint":  "私有化请用许可证开通模块；在线订购仅连我方 Server",
		})
		return
	}
	plans, err := a.Store.ListPlans()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	items := make([]map[string]any, 0, len(plans))
	for _, p := range plans {
		price := store.MockPlanPrice(p.Code)
		if price < 0 || p.Code == "free" {
			continue
		}
		items = append(items, map[string]any{
			"code": p.Code, "name": p.Name, "note": p.Note,
			"price": price, "currency": "CNY", "billing_cycle": "monthly_mock",
			"modules": p.Modules, "max_keys": p.MaxKeys,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items":  items,
		"provider": "mock",
		"hint":   "本刀为 Mock 支付：下单后调用 mock-pay 即开通，无真实收款通道",
	})
}

func (a *App) handleBillingOrders(w http.ResponseWriter, r *http.Request) {
	claims, err := a.requireJWT(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	if !a.isSaasMode() {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "subscription orders only in saas mode"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		list, err := a.Store.ListSubscriptionOrdersByUser(claims.UserID, 50)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": ordersJSON(list)})
	case http.MethodPost:
		var body struct {
			PlanCode string `json:"plan_code"`
			Method   string `json:"payment_method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad json"})
			return
		}
		o, err := a.Store.CreateSubscriptionOrder(claims.UserID, body.PlanCode, body.Method)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		a.Store.WriteAudit(claims.UserID, "order_create", o.OrderNo, o.PlanCode, clientIPFrom(r))
		writeJSON(w, http.StatusOK, map[string]any{
			"order": orderJSON(*o),
			"next":  "POST /api/billing/orders/mock-pay with {\"order_no\":\"" + o.OrderNo + "\"}",
		})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
	}
}

func (a *App) handleBillingMockPay(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	claims, err := a.requireJWT(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	if !a.isSaasMode() {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "mock pay only in saas mode"})
		return
	}
	var body struct {
		OrderNo string `json:"order_no"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.OrderNo) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "order_no required"})
		return
	}
	o, err := a.Store.GetSubscriptionOrderByNo(body.OrderNo)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "order not found"})
		return
	}
	// Owner or admin can mock-pay (admin for support demos).
	u, _ := a.Store.GetUser(claims.UserID)
	isAdmin := claims.IsAdmin || (u != nil && (u.Role == "admin" || a.isAdminUser(u.Username)))
	if o.UserID != claims.UserID && !isAdmin {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "not your order"})
		return
	}
	paid, err := a.Store.MockPaySubscription(body.OrderNo, claims.UserID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	a.Store.WriteAudit(claims.UserID, "order_paid_mock", paid.OrderNo, paid.PlanCode, clientIPFrom(r))
	writeJSON(w, http.StatusOK, map[string]any{
		"order":   orderJSON(*paid),
		"message": "mock payment accepted; plan activated",
	})
}

func (a *App) handleAdminBillingOrders(w http.ResponseWriter, r *http.Request) {
	claims, ok := a.requireAdminJWT(w, r)
	if !ok {
		return
	}
	if !a.isSaasMode() {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "admin billing orders only in saas"})
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	_ = claims
	list, err := a.Store.ListSubscriptionOrdersAdmin(100)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": ordersJSON(list), "provider": "mock"})
}

func orderJSON(o store.SubscriptionOrder) map[string]any {
	paid := ""
	if o.PaidAt.Valid {
		paid = o.PaidAt.String
	}
	return map[string]any{
		"id": o.ID, "order_no": o.OrderNo, "user_id": o.UserID, "plan_code": o.PlanCode,
		"amount": o.Amount, "currency": o.Currency, "payment_method": o.PaymentMethod,
		"status": o.Status, "paid_at": paid, "created_at": o.CreatedAt,
	}
}

func ordersJSON(list []store.SubscriptionOrder) []map[string]any {
	out := make([]map[string]any, 0, len(list))
	for _, o := range list {
		out = append(out, orderJSON(o))
	}
	return out
}
