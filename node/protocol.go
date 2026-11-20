package node

import (
	"errors"
	"math"
	"math/big"

	"github.com/Zenoglyphs-VM/zvm-core/release"
)

type ProtocolStatus struct {
	release.ProtocolSnapshot
	Blocked         bool                         `json:"blocked"`
	Reason          string                       `json:"reason,omitempty"`
	CheckedMomentum uint64                       `json:"checkedMomentum,omitempty"`
	Requirement     *release.ProtocolRequirement `json:"requirement,omitempty"`
}

// Once accepted derivation crosses a protocol boundary it stays held until a
// restart with a separately accepted release. Reorgs cannot clear this latch.
type protocolFailure struct {
	momentum    uint64
	reason      string
	requirement *release.ProtocolRequirement
}

func (f *protocolFailure) err() error {
	if f.requirement != nil {
		return &release.ProtocolHalt{Requirement: *f.requirement}
	}
	return errors.New(f.reason)
}

func (d *Deriver) holdProtocol(momentum uint64, err error) error {
	f := &protocolFailure{momentum: momentum, reason: err.Error()}
	var halt *release.ProtocolHalt
	if errors.As(err, &halt) {
		r := halt.Requirement
		f.requirement = &r
	}
	d.mu.Lock()
	if d.protocolFailure == nil {
		d.protocolFailure = f
	}
	f = d.protocolFailure
	d.mu.Unlock()
	return f.err()
}

func (d *Deriver) checkProtocol(momentum uint64) error {
	if d.Protocol == nil {
		return nil
	}
	d.mu.RLock()
	f := d.protocolFailure
	d.mu.RUnlock()
	if f != nil {
		return f.err()
	}
	if err := d.Protocol.Check(d.Cfg, momentum); err != nil {
		return d.holdProtocol(momentum, err)
	}
	return nil
}

func (d *Deriver) checkStoredProtocol() error {
	if err := d.checkProtocol(0); err != nil {
		return err
	}
	if d.Protocol == nil {
		return nil
	}
	head, err := d.DB.Head()
	if err != nil {
		return err
	}
	if head != nil {
		return d.checkProtocol(head.Height)
	}
	return nil
}

func (d *Deriver) checkEngineProtocol(number *big.Int) error {
	if d.Protocol == nil {
		return nil
	}
	if number == nil || !number.IsUint64() || number.Uint64() > math.MaxUint64-d.Cfg.L1StartHeight+1 {
		return d.holdProtocol(0, errors.New("engine height cannot be mapped to the accepted protocol configuration"))
	}
	return d.checkProtocol(d.Cfg.MomentumForL2Block(number.Uint64()))
}

func (d *Deriver) protocolStatus(height, frontier uint64) ProtocolStatus {
	s := ProtocolStatus{ProtocolSnapshot: d.Protocol.Snapshot()}
	if d.Protocol == nil {
		return s
	}
	checked := height
	if frontier > height && height < math.MaxUint64 {
		checked++
	}
	_ = d.checkProtocol(checked)
	d.mu.RLock()
	f := d.protocolFailure
	d.mu.RUnlock()
	if f != nil {
		s.Blocked = true
		s.Reason = f.reason
		s.CheckedMomentum = f.momentum
		if f.requirement != nil {
			r := *f.requirement
			s.Requirement = &r
		}
	}
	return s
}
