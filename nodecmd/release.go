package nodecmd

import (
	"errors"

	"github.com/Zenoglyphs-VM/zvm-core/release"
	"github.com/Zenoglyphs-VM/zvm-core/rollup"
)

// Validate before opening a database, reading a JWT or dialing a service. A
// configured declaration binds the very same bytes parsed into the running node.
func loadReleaseConfiguration(config string, paths [4]string) (*rollup.Config, *release.Assertion, error) {
	cfg, assertion, _, err := loadProtocolConfiguration(config, paths, "")
	return cfg, assertion, err
}

func loadProtocolConfiguration(config string, paths [4]string, protocolPath string) (*rollup.Config, *release.Assertion, *release.ProtocolGuard, error) {
	count := 0
	for _, name := range paths {
		if name != "" {
			count++
		}
	}
	if count == 0 {
		if protocolPath != "" {
			return nil, nil, nil, errors.New("protocol policy requires all four independent release inputs")
		}
		cfg, err := rollup.Load(config)
		return cfg, nil, nil, err
	}
	if count != len(paths) {
		return nil, nil, nil, errors.New("release manifest, policy, signatures and trusted keys must be configured together")
	}
	raw, err := release.ReadDocument(config)
	if err != nil {
		return nil, nil, nil, err
	}
	var documents [4][]byte
	for i, name := range paths {
		documents[i], err = release.ReadDocument(name)
		if err != nil {
			return nil, nil, nil, err
		}
	}
	var protocol []byte
	if protocolPath != "" {
		protocol, err = release.ReadProtocolPolicyDocument(protocolPath)
		if err != nil {
			return nil, nil, nil, err
		}
	}
	return release.DecodeProtocolConfiguration(documents[0], documents[1], documents[2], documents[3], raw, protocol)
}
