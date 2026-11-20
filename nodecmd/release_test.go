package nodecmd

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Zenoglyphs-VM/zvm-core/release"
)

func TestReleaseInputsAreAllOrNoneBeforeReadingConfiguration(t *testing.T) {
	for mask := 1; mask < 15; mask++ {
		var paths [4]string
		for i := range paths {
			if mask&(1<<i) != 0 {
				paths[i] = "missing-fixture"
			}
		}
		if _, _, err := loadReleaseConfiguration("missing-config", paths); err == nil || err.Error() != "release manifest, policy, signatures and trusted keys must be configured together" {
			t.Fatal("partial release inputs did not fail before filesystem access")
		}
	}
}

func TestReleaseConfigurationUsesTheExactVerifiedStartupBytes(t *testing.T) {
	raw, err := os.ReadFile("../zvm-core/web/test/fixtures/release-assertion.json")
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Assertion   release.Assertion
		TrustedKeys release.TrustedKeys
	}
	if json.Unmarshal(raw, &v) != nil {
		t.Fatal("bad public fixture")
	}
	message, err := base64.StdEncoding.DecodeString(v.Assertion.SigningMessage)
	if err != nil {
		t.Fatal(err)
	}
	manifest := bytes.TrimPrefix(message, []byte("ZVM release manifest v1\n"))
	m, err := release.DecodeManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	// Reconstruct the independently selected synthetic fixture policy. A real
	// deployment obtains this from review, never from a queried node/manifest.
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
	if fingerprint, err := p.Fingerprint(); err != nil || fingerprint != m.PolicySHA256 {
		t.Fatal("independent fixture policy differs from its signed pin")
	}
	dir := t.TempDir()
	write := func(name string, value any) string {
		t.Helper()
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		name = filepath.Join(dir, name)
		if err := os.WriteFile(name, data, 0600); err != nil {
			t.Fatal(err)
		}
		return name
	}
	manifestPath := filepath.Join(dir, "manifest.json")
	if os.WriteFile(manifestPath, manifest, 0600) != nil {
		t.Fatal("write manifest")
	}
	paths := [4]string{manifestPath, write("policy.json", p), write("signatures.json", v.Assertion.Signatures), write("keys.json", v.TrustedKeys)}
	config, err := os.ReadFile("../zvm-core/chains/l2/rollup.json")
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(dir, "rollup.json")
	if os.WriteFile(configPath, config, 0600) != nil {
		t.Fatal("write config")
	}
	cfg, a, err := loadReleaseConfiguration(configPath, paths)
	if err != nil || a == nil || cfg == nil || a.EpochSHA256 != v.Assertion.EpochSHA256 || a.RuntimeObserved {
		t.Fatalf("verified configuration did not bind: %v", err)
	}
	if os.WriteFile(configPath, append(config, '\n'), 0600) != nil {
		t.Fatal("change config")
	}
	if _, _, err := loadReleaseConfiguration(configPath, paths); err == nil {
		t.Fatal("a different but parse-equivalent configuration was accepted")
	}
	if _, a, err := loadReleaseConfiguration(configPath, [4]string{}); err != nil || a != nil {
		t.Fatal("unconfigured release mode fabricated an identity")
	}
}
