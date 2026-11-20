package node

import (
	"context"
	"errors"
	"math"
	"math/big"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/Zenoglyphs-VM/zvm-core/derive"
	"github.com/Zenoglyphs-VM/zvm-core/l2"
	"github.com/Zenoglyphs-VM/zvm-core/release"
	"github.com/Zenoglyphs-VM/zvm-core/store"
	"github.com/Zenoglyphs-VM/zvm-core/znn"
	"github.com/Zenoglyphs-VM/zvm-node/internal/testfixture"
)

func protocolNode(t *testing.T, height uint64) (*Deriver, *store.Head) {
	t.Helper()
	f := testfixture.NewProtocol(t, release.ProtocolRequirement{Rule: "future-rule", Version: 1, Momentum: 12})
	db, err := store.Open(filepath.Join(t.TempDir(), "node.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	m := &znn.Momentum{Height: height, Hash: znn.Hash{0: byte(height)}, Timestamp: f.Config.L2GenesisTimestamp + height*10}
	if err := db.RecordDerived(&store.Derived{Momentum: m, L2Number: height - 1, L2Hash: common.Hash{0: byte(height)}, Plan: &derive.Plan{Momentum: m}}, 100); err != nil {
		t.Fatal(err)
	}
	head, err := db.Head()
	if err != nil || head == nil {
		t.Fatal("missing fixture head")
	}
	d := &Deriver{Cfg: f.Config, Protocol: f.Guard, DB: db}
	d.setHead(*head)
	return d, head
}

func TestProtocolHaltPrecedesPlanningEngineAndPersistence(t *testing.T) {
	for _, height := range []uint64{11, 12, 13} {
		t.Run(new(big.Int).SetUint64(height).String(), func(t *testing.T) {
			d, head := protocolNode(t, height-1)
			if _, err := d.DB.SQL().Exec("DROP TABLE chunks"); err != nil {
				t.Fatal(err)
			}
			chunk := znn.Hash{0: 7}
			block := &znn.AccountBlock{BlockType: znn.BlockTypeUserSend, Address: znn.Address{19: 9}, ToAddress: d.Cfg.Inbox, Data: derive.EncodeAssemble([]znn.Hash{chunk})}
			block.Hash = block.ComputeHash()
			dm := &znn.DetailedMomentum{Momentum: &znn.Momentum{Height: height, Hash: znn.Hash{0: byte(height)}, PreviousHash: head.Hash, Timestamp: d.Cfg.L2GenesisTimestamp + height*10, Content: []znn.AccountHeader{{Hash: block.Hash}}}, Blocks: []*znn.AccountBlock{block}}
			d.lastProgress.Store(123)
			notified := false
			d.OnDerived = func(*store.Derived) { notified = true }
			err := d.deriveOne(context.Background(), dm)
			var halt *release.ProtocolHalt
			if height == 11 {
				if err == nil || !strings.Contains(err.Error(), "reading chunk") {
					t.Fatalf("supported pre-boundary planning not reached: %v", err)
				}
			} else if !errors.As(err, &halt) || halt.Requirement.Momentum != 12 {
				t.Fatalf("unsupported boundary reached planning or engine: %v", err)
			}
			persisted, err := d.DB.Head()
			if err != nil || persisted == nil || *persisted != *head || d.Head() != *head || notified || d.lastProgress.Load() != 123 {
				t.Fatal("failed operation changed a canonical/progress boundary")
			}
			var count int
			if err := d.DB.SQL().QueryRow("SELECT COUNT(*) FROM messages").Scan(&count); err != nil || count != 0 {
				t.Fatal("protocol hold persisted a message outcome")
			}
		})
	}
}

func TestProtocolInitQuarantinesUnsupportedStoredHeadBeforeAnyEngineCall(t *testing.T) {
	d, head := protocolNode(t, 12)
	rewound := false
	d.OnRewind = func(uint64) { rewound = true }
	var halt *release.ProtocolHalt
	if err := d.Init(context.Background()); !errors.As(err, &halt) {
		t.Fatalf("persisted unsupported head not quarantined: %v", err)
	}
	persisted, err := d.DB.Head()
	if err != nil || persisted == nil || *persisted != *head || d.Head() != *head || rewound {
		t.Fatal("unsupported restart rewound original evidence")
	}
}

type protocolEngineRPC struct {
	genesis, latest *types.Header
	calls           int
}

func (e *protocolEngineRPC) GetBlockByNumber(_ context.Context, number string, _ bool) (*types.Header, error) {
	e.calls++
	if number == "0x0" {
		return e.genesis, nil
	}
	return e.latest, nil
}

func TestProtocolInitQuarantinesUnsupportedEngineHeadBeforeReconciliation(t *testing.T) {
	d, head := protocolNode(t, 11)
	g, err := l2.BuildGenesis(d.Cfg, d.Cfg.L2GenesisTimestamp)
	if err != nil {
		t.Fatal(err)
	}
	service := &protocolEngineRPC{genesis: g.ToBlock().Header(), latest: &types.Header{Number: big.NewInt(11), Difficulty: big.NewInt(0)}}
	srv := rpc.NewServer()
	if err := srv.RegisterName("eth", service); err != nil {
		t.Fatal(err)
	}
	defer srv.Stop()
	d.Eth = ethclient.NewClient(rpc.DialInProc(srv))
	defer d.Eth.Close()
	rewound := false
	d.OnRewind = func(uint64) { rewound = true }
	var halt *release.ProtocolHalt
	if err := d.Init(context.Background()); !errors.As(err, &halt) || halt.Requirement.Momentum != 12 {
		t.Fatalf("unsupported engine head reached reconciliation: %v", err)
	}
	persisted, err := d.DB.Head()
	if err != nil || persisted == nil || *persisted != *head || d.Head() != *head || rewound || service.calls != 2 {
		t.Fatal("engine quarantine changed original store or queried NoM")
	}
}

func TestProtocolStatusIsImmediateAndItsHoldCannotBeClearedByReorg(t *testing.T) {
	d, _ := protocolNode(t, 11)
	d.lastProgress.Store(time.Now().UnixNano())
	d.l1Frontier.Store(11)
	if s := d.Status(); s.Protocol.Blocked || s.Stalled || !s.Protocol.Enforced {
		t.Fatal("healthy supported head refused")
	}
	d.l1Frontier.Store(12)
	s := d.Status()
	if s.Stalled || !s.Protocol.Blocked || s.Protocol.Requirement == nil || s.Protocol.Requirement.Momentum != 12 {
		t.Fatal("protocol stop waited for the stall timer")
	}
	s.Protocol.Requirement.Momentum = 1
	s.Protocol.Requirements[0].Momentum = 1
	d.l1Frontier.Store(10)
	d.setHead(store.Head{Height: 10})
	if err := d.checkProtocol(10); err == nil || d.Status().Protocol.Requirement.Momentum != 12 {
		t.Fatal("rewind or caller snapshot cleared the held boundary")
	}
}

func TestProtocolChangedConfigurationAndUnmappableEngineHeightsStayHeld(t *testing.T) {
	for _, name := range []string{"configuration", "engine height"} {
		t.Run(name, func(t *testing.T) {
			d, _ := protocolNode(t, 11)
			if name == "configuration" {
				d.Cfg.GasLimit++
				if err := d.checkProtocol(11); err == nil {
					t.Fatal("changed configuration accepted")
				}
				d.Cfg.GasLimit--
			} else if err := d.checkEngineProtocol(new(big.Int).SetUint64(math.MaxUint64)); err == nil {
				t.Fatal("overflowing engine mapping accepted")
			}
			if !d.Status().Protocol.Blocked || d.checkProtocol(1) == nil {
				t.Fatal("invalid state did not latch a protocol hold")
			}
		})
	}
}

func TestProtocolStatusAndDerivationChecksAreConcurrentSnapshots(t *testing.T) {
	d, _ := protocolNode(t, 11)
	d.l1Frontier.Store(12)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				_ = d.checkProtocol(12)
				s := d.Status()
				if !s.Protocol.Blocked {
					t.Error("concurrent check lost the hold")
				}
			}
		}()
	}
	wg.Wait()
}
