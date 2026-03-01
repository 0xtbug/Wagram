package repository_test

import (
	"os"
	"testing"

	"telegram-wa/internal/repository"
)

func newTestRepo(t *testing.T) *repository.SQLiteRepository {
	t.Helper()
	tmpFile := t.TempDir() + "/test.db"
	repo, err := repository.NewSQLiteRepository(tmpFile)
	if err != nil {
		t.Fatalf("failed to create test repo: %v", err)
	}
	t.Cleanup(func() {
		repo.Close()
		os.Remove(tmpFile)
	})
	return repo
}

func TestAdd_And_GetByTG(t *testing.T) {
	repo := newTestRepo(t)

	err := repo.Add("628123@s.whatsapp.net", 100)
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	waID, err := repo.GetByTG(100)
	if err != nil {
		t.Fatalf("GetByTG failed: %v", err)
	}
	if waID != "628123@s.whatsapp.net" {
		t.Errorf("expected 628123@s.whatsapp.net, got %q", waID)
	}
}

func TestGetByTG_NotFound(t *testing.T) {
	repo := newTestRepo(t)

	waID, err := repo.GetByTG(999)
	if err != nil {
		t.Fatalf("GetByTG failed: %v", err)
	}
	if waID != "" {
		t.Errorf("expected empty string for missing mapping, got %q", waID)
	}
}

func TestAdd_Upsert(t *testing.T) {
	repo := newTestRepo(t)

	repo.Add("old_jid@s.whatsapp.net", 200)

	repo.Add("new_jid@s.whatsapp.net", 200)

	waID, _ := repo.GetByTG(200)
	if waID != "new_jid@s.whatsapp.net" {
		t.Errorf("upsert failed: expected new_jid, got %q", waID)
	}
}

func TestGetByWA(t *testing.T) {
	repo := newTestRepo(t)

	repo.Add("628123@s.whatsapp.net", 100)
	repo.Add("628123@s.whatsapp.net", 200)
	repo.Add("628999@s.whatsapp.net", 300)

	ids, err := repo.GetByWA("628123@s.whatsapp.net")
	if err != nil {
		t.Fatalf("GetByWA failed: %v", err)
	}
	if len(ids) != 2 {
		t.Errorf("expected 2 TG chats, got %d", len(ids))
	}
}

func TestGetByWA_NotFound(t *testing.T) {
	repo := newTestRepo(t)

	ids, err := repo.GetByWA("nonexistent@s.whatsapp.net")
	if err != nil {
		t.Fatalf("GetByWA failed: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected 0 results, got %d", len(ids))
	}
}

func TestRemoveByTG(t *testing.T) {
	repo := newTestRepo(t)

	repo.Add("628123@s.whatsapp.net", 100)

	err := repo.RemoveByTG(100)
	if err != nil {
		t.Fatalf("RemoveByTG failed: %v", err)
	}

	waID, _ := repo.GetByTG(100)
	if waID != "" {
		t.Errorf("mapping should be deleted, got %q", waID)
	}
}

func TestRemoveByTG_NonExistent(t *testing.T) {
	repo := newTestRepo(t)

	err := repo.RemoveByTG(999)
	if err != nil {
		t.Fatalf("RemoveByTG should not error for non-existent: %v", err)
	}
}

func TestGetAll(t *testing.T) {
	repo := newTestRepo(t)

	repo.Add("jid_a@s.whatsapp.net", 100)
	repo.Add("jid_b@s.whatsapp.net", 200)
	repo.Add("jid_c@s.whatsapp.net", 300)

	mappings, err := repo.GetAll()
	if err != nil {
		t.Fatalf("GetAll failed: %v", err)
	}
	if len(mappings) != 3 {
		t.Errorf("expected 3 mappings, got %d", len(mappings))
	}
}

func TestGetAll_Empty(t *testing.T) {
	repo := newTestRepo(t)

	mappings, err := repo.GetAll()
	if err != nil {
		t.Fatalf("GetAll failed: %v", err)
	}
	if len(mappings) != 0 {
		t.Errorf("expected 0 mappings, got %d", len(mappings))
	}
}

func TestAdd_And_GetAll_VerifyFields(t *testing.T) {
	repo := newTestRepo(t)

	repo.Add("628123@s.whatsapp.net", 100)

	mappings, _ := repo.GetAll()
	if len(mappings) != 1 {
		t.Fatalf("expected 1 mapping, got %d", len(mappings))
	}

	m := mappings[0]
	if m.WAChatID != "628123@s.whatsapp.net" {
		t.Errorf("unexpected WAChatID: %q", m.WAChatID)
	}
	if m.TGChatID != 100 {
		t.Errorf("unexpected TGChatID: %d", m.TGChatID)
	}
}
