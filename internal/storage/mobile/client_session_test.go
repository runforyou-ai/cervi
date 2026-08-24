//go:build !server && (ios || android)

package mobile

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/runforyou-ai/cervi/internal/clientsession"
)

// TestClientSessionPersistsInMobileStorage 验证移动端登录凭据能够持久化和删除。
func TestClientSessionPersistsInMobileStorage(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "cervi.db")
	store, err := Open(context.Background(), databasePath)
	if err != nil {
		t.Fatal(err)
	}

	if _, found, err := store.LoadClientSession(context.Background()); err != nil || found {
		t.Fatalf("initial client session found = %v, error = %v", found, err)
	}
	expected := clientsession.Credential{
		ServerURL:      "https://cervi.example.com",
		OrganizationID: "organization-1",
		UserID:         "user-1",
		Token:          "test-token",
		ExpiresAt:      time.Now().UTC().Add(time.Hour),
	}
	if err := store.SaveClientSession(context.Background(), expected); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	store, err = Open(context.Background(), databasePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	actual, found, err := store.LoadClientSession(context.Background())
	if err != nil || !found {
		t.Fatalf("persisted client session found = %v, error = %v", found, err)
	}
	if actual.ServerURL != expected.ServerURL || actual.OrganizationID != expected.OrganizationID || actual.UserID != expected.UserID || actual.Token != expected.Token || !actual.ExpiresAt.Equal(expected.ExpiresAt) {
		t.Fatalf("client session = %#v, want %#v", actual, expected)
	}
	if err := store.DeleteClientSession(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, found, err := store.LoadClientSession(context.Background()); err != nil || found {
		t.Fatalf("deleted client session found = %v, error = %v", found, err)
	}
}
