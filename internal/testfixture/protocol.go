// Package testfixture supplies synthetic release inputs for tests only. None of
// its ephemeral keys or policy data may be used as operator trust material.
package testfixture

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	"github.com/Zenoglyphs-VM/zvm-core/release"
	"github.com/Zenoglyphs-VM/zvm-core/rollup"
)

type Protocol struct {
	Configuration []byte
	Documents     [4][]byte
	Policy        []byte
	Config        *rollup.Config
	Assertion     *release.Assertion
	Guard         *release.ProtocolGuard
}

func NewProtocol(t testing.TB, extra ...release.ProtocolRequirement) Protocol {
	t.Helper()
	return newProtocol(t, nil, extra...)
}

func NewIsthmusProtocol(t testing.TB, cfg *rollup.Config, extra ...release.ProtocolRequirement) Protocol {
	t.Helper()
	return newProtocol(t, cfg, extra...)
}

func newProtocol(t testing.TB, isthmus *rollup.Config, extra ...release.ProtocolRequirement) Protocol {
	t.Helper()
	_, name, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("fixture source unavailable")
	}
	core := filepath.Join(filepath.Dir(name), "..", "..", "zvm-core")
	read := func(name string) []byte {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(core, name))
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	marshal := func(value any) []byte {
		t.Helper()
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	digest := func(raw []byte) string { x := sha256.Sum256(raw); return hex.EncodeToString(x[:]) }
	var vector struct{ Assertion release.Assertion }
	if err := json.Unmarshal(read("web/test/fixtures/release-assertion.json"), &vector); err != nil {
		t.Fatal(err)
	}
	message, err := base64.StdEncoding.DecodeString(vector.Assertion.SigningMessage)
	if err != nil {
		t.Fatal(err)
	}
	m, err := release.DecodeManifest(bytes.TrimPrefix(message, []byte("ZVM release manifest v1\n")))
	if err != nil {
		t.Fatal(err)
	}
	// This reconstruction is specific to the public synthetic vector. Real
	// inventories are independently reviewed, never adopted from a manifest.
	p := release.Policy{Schema: 1, MinSignatures: 2}
	for _, r := range m.Repositories {
		p.Repositories = append(p.Repositories, r.Name)
	}
	for _, a := range m.Toolchains {
		p.Toolchains = append(p.Toolchains, a.ID)
	}
	for _, a := range m.Images {
		p.Images = append(p.Images, a.ID)
	}
	for _, a := range m.Compilers {
		p.Compilers = append(p.Compilers, a.ID)
	}
	for _, c := range m.Contracts {
		p.Contracts = append(p.Contracts, c.ID)
	}
	for _, f := range m.Files {
		p.Files = append(p.Files, release.FileRequirement{ID: f.ID, Kind: f.Kind, Repository: f.Repository, Path: f.Path})
	}
	requirements := []release.ProtocolRequirement{
		{Rule: "derivation", Version: 1, Momentum: 2}, {Rule: "evm-karst", Version: 1, Momentum: 2},
		{Rule: "native-fee", Version: 1, Momentum: 10}, {Rule: "gas", Version: 1, Momentum: 200},
		{Rule: "wire", Version: 1, Momentum: 200}, {Rule: "fee-market", Version: 1, Momentum: 1440},
	}
	var configuration []byte
	if isthmus == nil {
		configuration = read("chains/l2/rollup.json")
	} else {
		configuration = marshal(isthmus)
		m.Epoch.L1ChainID = strconv.FormatUint(isthmus.L1ChainID, 10)
		m.Epoch.L2ChainID = strconv.FormatUint(isthmus.L2ChainID, 10)
		m.Epoch.L2GenesisHash = isthmus.L2GenesisHash.Hex()
		m.Epoch.RollupSHA256 = digest(configuration)
		m.Epoch.EVM = rollup.EVMIsthmus
		for i := range m.Files {
			if m.Files[i].Kind == "rollup" {
				m.Files[i].SHA256 = m.Epoch.RollupSHA256
				m.Files[i].Bytes = int64(len(configuration))
			}
		}
		requirements = []release.ProtocolRequirement{
			{Rule: "derivation", Version: 1, Momentum: isthmus.L1StartHeight},
			{Rule: "evm-isthmus", Version: 1, Momentum: isthmus.L1StartHeight},
		}
	}
	policy := release.ProtocolActivationPolicy{
		Schema: 1, EpochID: m.Epoch.ID, ChainID: m.Epoch.L2ChainID,
		GenesisHash: m.Epoch.L2GenesisHash, RollupSHA256: m.Epoch.RollupSHA256,
		Requirements: append(requirements, extra...),
	}
	raw := marshal(policy)
	f := release.File{ID: release.ProtocolPolicyFileID, Kind: "config", Repository: "zvm-core", Path: "protocol.fixture.json", SHA256: digest(raw), Bytes: int64(len(raw))}
	m.Files = append(m.Files, f)
	p.Files = append(p.Files, release.FileRequirement{ID: f.ID, Kind: f.Kind, Repository: f.Repository, Path: f.Path})
	m.PolicySHA256, err = p.Fingerprint()
	if err != nil {
		t.Fatal(err)
	}
	message, err = m.SigningMessage()
	if err != nil {
		t.Fatal(err)
	}
	keys := release.TrustedKeys{Schema: 1}
	signatures := release.Signatures{Schema: 1}
	for i := 0; i < 2; i++ {
		public, private, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		keys.Keys = append(keys.Keys, base64.StdEncoding.EncodeToString(public))
		signatures.Signatures = append(signatures.Signatures, release.Signature{Algorithm: "ed25519", KeySHA256: digest(public), Value: base64.StdEncoding.EncodeToString(ed25519.Sign(private, message))})
	}
	out := Protocol{Configuration: configuration, Documents: [4][]byte{marshal(m), marshal(p), marshal(signatures), marshal(keys)}, Policy: raw}
	out.Config, out.Assertion, out.Guard, err = release.DecodeProtocolConfiguration(out.Documents[0], out.Documents[1], out.Documents[2], out.Documents[3], out.Configuration, out.Policy)
	if err != nil || out.Guard == nil {
		t.Fatalf("invalid synthetic protocol fixture: %v", err)
	}
	return out
}
