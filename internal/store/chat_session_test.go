package store

import (
	"path/filepath"
	"testing"
)

func TestChatSessionRoundTrip(t *testing.T) {
	s, err := Open("sqlite", filepath.Join(t.TempDir(), "t.db"), "")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	id, err := s.CreateChatSession(7, "播报一下今天的国际要闻", "deepseek-flash")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AppendChatTurn(7, id, "你好", "你好，我是助手", "mock-chat"); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendChatTurn(7, id, "继续", "接着说", "deepseek-flash"); err != nil {
		t.Fatal(err)
	}
	msgs, err := s.ListChatMessages(7, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 4 || msgs[3].ModelName != "deepseek-flash" {
		t.Fatalf("msgs %+v", msgs)
	}
	if _, err := s.ListChatMessages(8, id); err != ErrNotFound {
		t.Fatalf("other user err %v", err)
	}
	list, err := s.ListChatSessions(7, 10)
	if err != nil || len(list) != 1 || list[0].Title == "" {
		t.Fatalf("list %+v %v", list, err)
	}
	if err := s.DeleteChatSession(7, id); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetChatSession(7, id); err != ErrNotFound {
		t.Fatal(err)
	}
}
