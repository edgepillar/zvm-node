package nodecmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Zenoglyphs-VM/zvm-core/release"
	"github.com/Zenoglyphs-VM/zvm-node/internal/testfixture"
)

func TestProtocolStartupRequiresIndependentReleaseInputsBeforeFileAccess(t *testing.T) {
	if _, _, _, err := loadProtocolConfiguration("missing", [4]string{}, "missing-policy"); err == nil || err.Error() != "protocol policy requires all four independent release inputs" {
		t.Fatal("unsigned protocol policy reached the filesystem")
	}
	for mask := 1; mask < 15; mask++ {
		var paths [4]string
		for i := range paths {
			if mask&(1<<i) != 0 {
				paths[i] = "missing"
			}
		}
		if _, _, _, err := loadProtocolConfiguration("missing", paths, "missing-policy"); err == nil || !strings.Contains(err.Error(), "must be configured together") {
			t.Fatal("partial release inputs reached the filesystem")
		}
	}
}

func TestProtocolStartupUsesSignedSnapshotAndRejectsPolicyChangesBeforeJWTAndDatabase(t *testing.T) {
	f := testfixture.NewProtocol(t, release.ProtocolRequirement{Rule: "future-rule", Version: 1, Momentum: 12})
	dir := t.TempDir()
	write := func(name string, raw []byte) string {
		t.Helper()
		name = filepath.Join(dir, name)
		if err := os.WriteFile(name, raw, 0600); err != nil {
			t.Fatal(err)
		}
		return name
	}
	config := write("rollup.json", f.Configuration)
	protocol := write("protocol.json", f.Policy)
	var paths [4]string
	for i, name := range []string{"manifest.json", "inventory.json", "signatures.json", "keys.json"} {
		paths[i] = write(name, f.Documents[i])
	}
	cfg, a, g, err := loadProtocolConfiguration(config, paths, protocol)
	if err != nil || cfg == nil || a == nil || g == nil || !g.Snapshot().Enforced || g.Check(cfg, 11) != nil || g.Check(cfg, 12) == nil {
		t.Fatalf("accepted startup guard missing: %v", err)
	}
	if _, _, err := loadReleaseConfiguration(config, paths); err == nil {
		t.Fatal("declared policy was silently omitted")
	}
	if err := os.WriteFile(protocol, append(f.Policy, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	db := filepath.Join(dir, "must-not-open.db")
	err = Run([]string{"-config", config, "-release-manifest", paths[0], "-release-policy", paths[1], "-release-signatures", paths[2], "-release-trusted-keys", paths[3], "-protocol-policy", protocol, "-jwt", filepath.Join(dir, "must-not-read.jwt"), "-db", db, "-mainnet-rpc", "off"})
	if err == nil || err.Error() != "protocol policy bytes disagree with the accepted release" {
		t.Fatalf("startup did not reject protocol bytes before JWT or dialing: %v", err)
	}
	if _, err := os.Stat(db); !os.IsNotExist(err) {
		t.Fatal("invalid startup policy opened a database")
	}
}
