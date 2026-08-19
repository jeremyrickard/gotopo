package gotopo

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestHostedRequestSigningAndPayload(t *testing.T) {
	key := base64.StdEncoding.EncodeToString([]byte("secret"))
	fixed := time.UnixMilli(1_700_000_000_000)
	var received url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/map/ABC/Marker" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		received, _ = url.ParseQuery(string(body))
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"status":"ok","result":{"id":"feature-1","properties":{"class":"Marker","title":"test"}}}`)
	}))
	defer server.Close()

	client, err := NewClient(WithEndpoint(server.URL), WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	client.hosted = true
	client.credentials = Credentials{ID: "ABCDEFGHIJKL", Key: key}
	client.clock = func() time.Time { return fixed }
	client.mapID = "ABC"

	path, err := client.mapPath("Marker", "")
	if err != nil {
		t.Fatal(err)
	}
	payload := Feature{Properties: Properties{"title": "test"}}
	var result Feature
	if err := client.do(context.Background(), requestSpec{method: http.MethodPost, path: path, payload: payload}, &result); err != nil {
		t.Fatal(err)
	}
	if received.Get("id") != "ABCDEFGHIJKL" || received.Get("expires") != "1700000120000" {
		t.Fatalf("missing signing fields: %v", received)
	}
	rawJSON := received.Get("json")
	var decoded Feature
	if err := json.Unmarshal([]byte(rawJSON), &decoded); err != nil {
		t.Fatalf("invalid json form field: %v", err)
	}
	wantSignature, err := signRequest(key, http.MethodPost, path, 1_700_000_120_000, []byte(rawJSON))
	if err != nil {
		t.Fatal(err)
	}
	if received.Get("signature") != wantSignature {
		t.Fatalf("signature mismatch: got %q want %q", received.Get("signature"), wantSignature)
	}
}

func TestAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "denied", http.StatusUnauthorized)
	}))
	defer server.Close()
	client, err := NewClient(WithEndpoint(server.URL), WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	var out Feature
	err = client.do(context.Background(), requestSpec{method: http.MethodGet, path: "/failure"}, &out)
	apiErr, ok := err.(*APIError)
	if !ok || apiErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 APIError, got %T %v", err, err)
	}
}
