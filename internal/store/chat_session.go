package store

import (
	"database/sql"
	"errors"
	"strings"
)

type ChatSession struct {
	ID        int64
	UserID    int64
	Title     string
	ModelName string
	CreatedAt string
	UpdatedAt string
}

type ChatTurn struct {
	ID        int64
	SessionID int64
	Role      string
	Content   string
	ModelName string
	CreatedAt string
}

func chatTitle(text string) string {
	text = strings.TrimSpace(text)
	text = strings.ReplaceAll(text, "\n", " ")
	r := []rune(text)
	if len(r) == 0 {
		return "新对话"
	}
	if len(r) > 24 {
		return string(r[:24]) + "…"
	}
	return string(r)
}

func (s *Store) CreateChatSession(userID int64, title, modelName string) (int64, error) {
	now := Now()
	res, err := s.DB.Exec(`INSERT INTO chat_session(user_id,title,model_name,created_at,updated_at) VALUES(?,?,?,?,?)`,
		userID, chatTitle(title), modelName, now, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) ListChatSessions(userID int64, limit int) ([]ChatSession, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.DB.Query(`SELECT id, user_id, title, model_name, created_at, updated_at
FROM chat_session WHERE user_id=? ORDER BY updated_at DESC, id DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ChatSession
	for rows.Next() {
		var c ChatSession
		if err := rows.Scan(&c.ID, &c.UserID, &c.Title, &c.ModelName, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) GetChatSession(userID, id int64) (*ChatSession, error) {
	row := s.DB.QueryRow(`SELECT id, user_id, title, model_name, created_at, updated_at FROM chat_session WHERE id=? AND user_id=?`, id, userID)
	var c ChatSession
	if err := row.Scan(&c.ID, &c.UserID, &c.Title, &c.ModelName, &c.CreatedAt, &c.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &c, nil
}

func (s *Store) ListChatMessages(userID, sessionID int64) ([]ChatTurn, error) {
	if _, err := s.GetChatSession(userID, sessionID); err != nil {
		return nil, err
	}
	rows, err := s.DB.Query(`SELECT id, session_id, role, content, model_name, created_at
FROM chat_message WHERE session_id=? ORDER BY id ASC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ChatTurn
	for rows.Next() {
		var m ChatTurn
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &m.ModelName, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) AppendChatTurn(userID, sessionID int64, userText, answer, modelName string) error {
	if _, err := s.GetChatSession(userID, sessionID); err != nil {
		return err
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	now := Now()
	if _, err := tx.Exec(`INSERT INTO chat_message(session_id,role,content,model_name,created_at) VALUES(?,?,?,?,?)`,
		sessionID, "user", userText, "", now); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO chat_message(session_id,role,content,model_name,created_at) VALUES(?,?,?,?,?)`,
		sessionID, "assistant", answer, modelName, now); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE chat_session SET model_name=?, updated_at=? WHERE id=? AND user_id=?`,
		modelName, now, sessionID, userID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) DeleteChatSession(userID, id int64) error {
	if _, err := s.GetChatSession(userID, id); err != nil {
		return err
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`DELETE FROM chat_message WHERE session_id=?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM chat_session WHERE id=? AND user_id=?`, id, userID); err != nil {
		return err
	}
	return tx.Commit()
}
