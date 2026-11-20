package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Zenoglyphs-VM/zvm-node/internal/testfixture"
	"github.com/Zenoglyphs-VM/zvm-node/node"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

type protocolExtension struct{}

func (protocolExtension) Register(*http.ServeMux) {}
func (protocolExtension) Info() map[string]any {
	return map[string]any{"protocol": map[string]any{"blocked": false, "enforced": false}}
}
func (protocolExtension) Wrap(h http.Handler) http.Handler { return h }

type offlineProtocolEth struct{}

func (offlineProtocolEth) GetBlockByNumber(context.Context, string, bool) (*types.Header, error) {
	return nil, errors.New("synthetic offline engine")
}

func TestProtocolHealthAndInfoExposeImmediateHoldDespiteExtensionFields(t *testing.T) {
	f := testfixture.NewProtocol(t)
	d := &node.Deriver{Cfg: f.Config, Protocol: f.Guard}
	srv := rpc.NewServer()
	if err := srv.RegisterName("eth", offlineProtocolEth{}); err != nil {
		t.Fatal(err)
	}
	defer srv.Stop()
	eth := ethclient.NewClient(rpc.DialInProc(srv))
	defer eth.Close()
	s := &Server{cfg: Config{Public: true, Rollup: f.Config, Deriver: d, Eth: eth, Extensions: []Extension{protocolExtension{}}}}
	h := s.Handler()
	request := func(path string) (int, map[string]json.RawMessage) {
		t.Helper()
		r := httptest.NewRecorder()
		h.ServeHTTP(r, httptest.NewRequest(http.MethodGet, path, nil))
		var body map[string]json.RawMessage
		if err := json.Unmarshal(r.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		return r.Code, body
	}
	code, body := request("/healthz")
	var healthy bool
	if json.Unmarshal(body["ok"], &healthy) != nil || !healthy || code != 200 {
		t.Fatal("supported configured node was not healthy")
	}
	f.Config.GasLimit++
	for _, path := range []string{"/healthz", "/api/info"} {
		code, body = request(path)
		var protocol node.ProtocolStatus
		if json.Unmarshal(body["protocol"], &protocol) != nil || !protocol.Enforced || !protocol.Blocked || protocol.Reason != "accepted protocol configuration changed; derivation is stopped" {
			t.Fatalf("safety state absent or replaced at %s", path)
		}
		if path == "/healthz" {
			if code != 503 || json.Unmarshal(body["ok"], &healthy) != nil || healthy {
				t.Fatal("protocol hold waited for stalled derivation")
			}
		}
	}
}

func TestProtocolLegacyHealthReportsEnforcementDisabled(t *testing.T) {
	f := testfixture.NewProtocol(t)
	s := &Server{cfg: Config{Deriver: &node.Deriver{Cfg: f.Config}}}
	r := httptest.NewRecorder()
	s.healthz(r, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	var body struct{ Protocol node.ProtocolStatus }
	if json.Unmarshal(r.Body.Bytes(), &body) != nil || r.Code != 200 || body.Protocol.Enforced || body.Protocol.Blocked || len(body.Protocol.Capabilities) != 7 {
		t.Fatal("legacy mode fabricated protocol enforcement")
	}
}
