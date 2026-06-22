package session

import (
	"context"
	"path/filepath"
	"testing"

	"go-backend/internal/llm"
	"go-backend/internal/storage"
)

func TestSQLiteServicePersistsSessionMessages(t *testing.T) {
	db, err := storage.OpenSQLite(storage.SQLiteConfig{
		Path: filepath.Join(t.TempDir(), "data.db"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	service := NewSQLiteService(db)
	ctx := context.Background()

	sess, err := service.Create(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if sess.ID == "" {
		t.Fatal("expected session id")
	}

	if err := service.AppendMessage(ctx, sess.ID, Message{
		Role:    llm.RoleUser,
		Content: "hello",
	}); err != nil {
		t.Fatal(err)
	}
	if err := service.AppendMessage(ctx, sess.ID, Message{
		Role:    llm.RoleAssistant,
		Content: "hi",
	}); err != nil {
		t.Fatal(err)
	}

	messages, err := service.ListMessages(ctx, sess.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if messages[0].Role != llm.RoleUser || messages[0].Content != "hello" {
		t.Fatalf("unexpected first message: %#v", messages[0])
	}
	if messages[1].Role != llm.RoleAssistant || messages[1].Content != "hi" {
		t.Fatalf("unexpected second message: %#v", messages[1])
	}

	loaded, err := service.Get(ctx, sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ID != sess.ID {
		t.Fatalf("expected session %q, got %q", sess.ID, loaded.ID)
	}
}

func TestSQLiteServiceListAndDeleteSessions(t *testing.T) {
	db, err := storage.OpenSQLite(storage.SQLiteConfig{
		Path: filepath.Join(t.TempDir(), "data.db"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	service := NewSQLiteService(db)
	ctx := context.Background()

	first, err := service.Create(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.AppendMessage(ctx, first.ID, Message{
		Role:    llm.RoleUser,
		Content: "hello",
	}); err != nil {
		t.Fatal(err)
	}
	second, err := service.Create(ctx)
	if err != nil {
		t.Fatal(err)
	}

	list, err := service.List(ctx, ListOptions{Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Sessions) != 1 || !list.HasMore || list.NextCursor == "" {
		t.Fatalf("unexpected first page: %#v", list)
	}

	next, err := service.List(ctx, ListOptions{Limit: 10, Cursor: list.NextCursor})
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Sessions) != 1 {
		t.Fatalf("unexpected second page: %#v", next)
	}

	if err := service.Delete(ctx, second.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Get(ctx, second.ID); err != ErrSessionNotFound {
		t.Fatalf("expected ErrSessionNotFound after delete, got %v", err)
	}
}
