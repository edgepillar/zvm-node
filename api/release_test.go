package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/Zenoglyphs-VM/zvm-core/release"
)

func TestReleaseRouteFreezesPublicAssertionWithoutClaimingRuntime(t *testing.T) {
	raw, err := os.ReadFile("../zvm-core/web/test/fixtures/release-assertion.json")
	if err != nil {
		t.Fatal(err)
	}
	var vector struct{ Assertion release.Assertion }
	if err := json.Unmarshal(raw, &vector); err != nil {
		t.Fatal(err)
	}
	s := &Server{cfg: Config{Public: true, Release: &vector.Assertion}}
	h := s.Handler()
	wanted := vector.Assertion.ManifestSHA256
	vector.Assertion.ManifestSHA256 = "changed"
	vector.Assertion.Signatures.Signatures[0].Value = "changed"
	r := httptest.NewRecorder()
	h.ServeHTTP(r, httptest.NewRequest(http.MethodGet, "/api/release", nil))
	if r.Code != http.StatusOK || r.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("wrong release response: %d", r.Code)
	}
	var got release.Assertion
	if err := json.Unmarshal(r.Body.Bytes(), &got); err != nil || got.ManifestSHA256 != wanted || got.RuntimeObserved || got.Signatures.Signatures[0].Value == "changed" {
		t.Fatal("public assertion changed after handler construction or claimed runtime observation")
	}
	var fields map[string]any
	if json.Unmarshal(r.Body.Bytes(), &fields) != nil || len(fields) != 6 {
		t.Fatal("unexpected fields in public release response")
	}
}

func TestReleaseRouteRefusesMissingDeclarationAndWriteMethods(t *testing.T) {
	s := &Server{cfg: Config{Public: true}}
	h := s.Handler()
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		r := httptest.NewRecorder()
		h.ServeHTTP(r, httptest.NewRequest(method, "/api/release", nil))
		if method == http.MethodGet && r.Code != http.StatusServiceUnavailable {
			t.Fatalf("missing identity returned %d", r.Code)
		}
		if method == http.MethodPost && r.Code < 400 {
			t.Fatalf("write method accepted with %d", r.Code)
		}
	}
}
