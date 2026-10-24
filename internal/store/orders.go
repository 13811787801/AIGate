package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

type SubscriptionOrder struct {
	ID            int64
	OrderNo       string
	UserID        int64
	PlanCode      string
	Amount        float64
	Currency      string
	PaymentMethod string
	Status        string
	PaidAt        sql.NullString
	CreatedAt     string
}

// MockPlanPrice is the SaaS catalog price (CNY) until real payment SKUs land.
func MockPlanPrice(code string) float64 {
	switch strings.ToLower(strings.TrimSpace(code)) {
	case "pro":
		return 99
	case "enterprise":
		return 999
	case "free":
		return 0
	default:
		return -1
	}
}

func newOrderNo() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "ORD" + strings.ToUpper(hex.EncodeToString(b)), nil
}

func (s *Store) CreateSubscriptionOrder(userID int64, planCode, method string) (*SubscriptionOrder, error) {
	planCode = strings.ToLower(strings.TrimSpace(planCode))
	if planCode == "" || planCode == "free" {
		return nil, fmt.Errorf("plan_code required (not free)")
	}
	plan, err := s.GetPlanByCode(planCode)
	if err != nil {
		return nil, fmt.Errorf("unknown plan: %s", planCode)
	}
	if plan.Status != 1 {
		return nil, fmt.Errorf("plan disabled")
	}
	amount := MockPlanPrice(planCode)
	if amount < 0 {
		return nil, fmt.Errorf("no mock price for plan %s", planCode)
	}
	if method == "" {
		method = "mock"
	}
	orderNo, err := newOrderNo()
	if err != nil {
		return nil, err
	}
	now := Now()
	res, err := s.DB.Exec(`INSERT INTO subscription_order(order_no,user_id,plan_code,amount,currency,payment_method,status,created_at)
VALUES(?,?,?,?,?,?,?,?)`, orderNo, userID, planCode, amount, "CNY", method, "PENDING", now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &SubscriptionOrder{
		ID: id, OrderNo: orderNo, UserID: userID, PlanCode: planCode,
		Amount: amount, Currency: "CNY", PaymentMethod: method, Status: "PENDING", CreatedAt: now,
	}, nil
}

func (s *Store) GetSubscriptionOrderByNo(orderNo string) (*SubscriptionOrder, error) {
	row := s.DB.QueryRow(`SELECT id,order_no,user_id,plan_code,amount,currency,payment_method,status,paid_at,created_at
FROM subscription_order WHERE order_no=?`, orderNo)
	var o SubscriptionOrder
	if err := row.Scan(&o.ID, &o.OrderNo, &o.UserID, &o.PlanCode, &o.Amount, &o.Currency, &o.PaymentMethod, &o.Status, &o.PaidAt, &o.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &o, nil
}

func (s *Store) ListSubscriptionOrdersByUser(userID int64, limit int) ([]SubscriptionOrder, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.DB.Query(`SELECT id,order_no,user_id,plan_code,amount,currency,payment_method,status,paid_at,created_at
FROM subscription_order WHERE user_id=? ORDER BY id DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanOrders(rows)
}

func (s *Store) ListSubscriptionOrdersAdmin(limit int) ([]SubscriptionOrder, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.DB.Query(`SELECT id,order_no,user_id,plan_code,amount,currency,payment_method,status,paid_at,created_at
FROM subscription_order ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanOrders(rows)
}

func scanOrders(rows *sql.Rows) ([]SubscriptionOrder, error) {
	var out []SubscriptionOrder
	for rows.Next() {
		var o SubscriptionOrder
		if err := rows.Scan(&o.ID, &o.OrderNo, &o.UserID, &o.PlanCode, &o.Amount, &o.Currency, &o.PaymentMethod, &o.Status, &o.PaidAt, &o.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// MockPaySubscription marks order PAID and assigns plan to the user (idempotent if already PAID).
func (s *Store) MockPaySubscription(orderNo string, actorUserID int64) (*SubscriptionOrder, error) {
	o, err := s.GetSubscriptionOrderByNo(orderNo)
	if err != nil {
		return nil, err
	}
	if o.Status == "PAID" {
		return o, nil
	}
	if o.Status != "PENDING" {
		return nil, fmt.Errorf("order status is %s", o.Status)
	}
	plan, err := s.GetPlanByCode(o.PlanCode)
	if err != nil {
		return nil, err
	}
	now := Now()
	tx, err := s.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.Exec(`UPDATE subscription_order SET status='PAID', paid_at=?, payment_method='mock' WHERE order_no=? AND status='PENDING'`, now, orderNo)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		// concurrent pay
		_ = tx.Rollback()
		return s.GetSubscriptionOrderByNo(orderNo)
	}
	if _, err := tx.Exec(`UPDATE `+tblUser+` SET plan_id=?, updated_at=? WHERE id=?`, plan.ID, now, o.UserID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(`INSERT INTO audit_log(actor_user_id,action,target,detail,ip,created_at) VALUES(?,?,?,?,?,?)`,
		nullInt64(actorUserID), "mock_pay", orderNo, "plan="+o.PlanCode, "", now); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	o.Status = "PAID"
	o.PaidAt = sql.NullString{String: now, Valid: true}
	o.PaymentMethod = "mock"
	return o, nil
}
