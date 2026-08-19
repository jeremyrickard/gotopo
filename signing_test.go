package gotopo

import (
	"encoding/base64"
	"testing"
)

func TestSignRequest(t *testing.T) {
	key := base64.StdEncoding.EncodeToString([]byte("secret"))
	got, err := signRequest(key, "post", "/api/v1/map/ABC/Marker", 1234, []byte(`{"a":1}`))
	if err != nil {
		t.Fatal(err)
	}
	const want = "GL+ZiX2KOeCeQ1k42nHuEfbzoMOqwCNYAjRvo61OKOM="
	if got != want {
		t.Fatalf("signature mismatch: got %q want %q", got, want)
	}
}
