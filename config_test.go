package gotopo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCredentialsFromINI(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentials.ini")
	content := "[other]\nid=nope\n\n[user@example.com]\nid=ABCDEFGHIJKL\nkey=c2VjcmV0\naccountId=ABC123\naccountIdInternet=XYZ789\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := CredentialsFromINI(path, "user@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "ABCDEFGHIJKL" || got.Key != "c2VjcmV0" || got.AccountID != "ABC123" || got.InternetAccountID != "XYZ789" {
		t.Fatalf("unexpected credentials: %#v", got)
	}
}
