package api

import (
	"encoding/json"
	"net/http"

	"github.com/Zenoglyphs-VM/zvm-core/release"
)

// Freeze the declaration when the handler is constructed. Extension Info maps
// do not participate in this response; clients provision their own trust keys.
func releaseHandler(assertion *release.Assertion) http.Handler {
	status := http.StatusServiceUnavailable
	raw := []byte(`{"error":"signed release identity unavailable"}`)
	if assertion != nil {
		if encoded, err := json.Marshal(assertion); err == nil {
			raw, status = encoded, http.StatusOK
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(status)
		_, _ = w.Write(raw)
	})
}
