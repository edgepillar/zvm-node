package node

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/eth/catalyst"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/miner"
	gethnode "github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/Zenoglyphs-VM/zvm-core/l2"
	"github.com/Zenoglyphs-VM/zvm-core/release"
	"github.com/Zenoglyphs-VM/zvm-core/rollup"
	"github.com/Zenoglyphs-VM/zvm-core/store"
	"github.com/Zenoglyphs-VM/zvm-core/znn"
	"github.com/Zenoglyphs-VM/zvm-node/internal/testfixture"
)

type protocolExecution struct {
	deriver      *Deriver
	backend      *eth.Ethereum
	requests     *atomic.Uint64
	databasePath string
	derived      int
}

func openProtocolExecution(t *testing.T, ctx context.Context, genesis *core.Genesis, fixture testfixture.Protocol, nom string) *protocolExecution {
	t.Helper()
	stack, err := gethnode.New(&gethnode.Config{P2P: p2p.Config{NoDiscovery: true, NoDial: true}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := stack.Close(); err != nil {
			t.Error(err)
		}
	})
	backend, err := eth.New(stack, &ethconfig.Config{
		Genesis: genesis, SyncMode: ethconfig.FullSync, Miner: miner.DefaultConfig,
		TrieCleanCache: 16, TrieDirtyCache: 16, TrieTimeout: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := stack.Start(); err != nil {
		t.Fatal(err)
	}
	backend.SetSynced()
	server := rpc.NewServer()
	t.Cleanup(server.Stop)
	if err := server.RegisterName("engine", catalyst.NewConsensusAPI(backend)); err != nil {
		t.Fatal(err)
	}
	var requests atomic.Uint64
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		server.ServeHTTP(w, r)
	}))
	t.Cleanup(httpServer.Close)
	engine, err := l2.DialEngine(ctx, httpServer.URL, [32]byte{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(engine.Close)
	ethRPC := ethclient.NewClient(stack.Attach())
	t.Cleanup(ethRPC.Close)
	databasePath := filepath.Join(t.TempDir(), "protocol-node.db")
	db, err := store.Open(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	d := &Deriver{Cfg: fixture.Config, Protocol: fixture.Guard, L1: znn.NewClient(nom), Engine: engine, Eth: ethRPC,
		DB: db, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	run := &protocolExecution{deriver: d, backend: backend, requests: &requests, databasePath: databasePath}
	d.OnDerived = func(*store.Derived) { run.derived++ }
	if err := d.Init(ctx); err != nil {
		t.Fatal(err)
	}
	return run
}

func TestProtocolExecutionHoldsAtSignedBoundary(t *testing.T) {
	const boundary = 4
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cfg := &rollup.Config{L1ChainID: 70, L2ChainID: 7340470, Inbox: rollup.Inbox,
		Vault: znn.Address{19: 163}, EVM: rollup.EVMIsthmus, L1StartHeight: 2,
		L2GenesisTimestamp: 1_700_000_000, GasLimit: 30_000_000, MaxTxGas: 30_000_000,
		ChunkTTL: 100, FinalityDepth: 100}
	genesis, err := l2.BuildGenesis(cfg, cfg.L2GenesisTimestamp)
	if err != nil {
		t.Fatal(err)
	}
	cfg.L2GenesisHash = genesis.ToBlock().Hash()
	requirement := release.ProtocolRequirement{Rule: "future-fixture-rule", Version: 1, Momentum: boundary}
	controlFixture := testfixture.NewIsthmusProtocol(t, cfg)
	heldFixture := testfixture.NewIsthmusProtocol(t, cfg, requirement)
	if !bytes.Equal(controlFixture.Configuration, heldFixture.Configuration) ||
		controlFixture.Assertion.EpochSHA256 != heldFixture.Assertion.EpochSHA256 ||
		controlFixture.Guard.Snapshot().PolicySHA256 == heldFixture.Guard.Snapshot().PolicySHA256 {
		t.Fatal("fixtures must share configuration and epoch but have different signed policies")
	}
	var frontier atomic.Uint64
	frontier.Store(1)
	momentums := make([]*znn.DetailedMomentum, boundary+2)
	for height := uint64(1); height <= boundary+1; height++ {
		momentums[height] = &znn.DetailedMomentum{Momentum: &znn.Momentum{
			Height: height, Hash: znn.Hash{0: byte(height)}, PreviousHash: znn.Hash{0: byte(height - 1)},
			Timestamp: cfg.L2GenesisTimestamp + (height-1)*10,
		}}
	}
	nom := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var call struct {
			ID     json.RawMessage
			Method string
			Params []uint64
		}
		if json.NewDecoder(io.LimitReader(r.Body, 8<<10)).Decode(&call) != nil {
			http.Error(w, "unexpected synthetic NoM request", http.StatusBadRequest)
			return
		}
		head := frontier.Load()
		var result any
		switch call.Method {
		case "ledger.getFrontierMomentum":
			result = momentums[head].Momentum
		case "ledger.getDetailedMomentumsByHeight":
			if len(call.Params) == 2 && call.Params[0] > 0 && call.Params[0] <= head && call.Params[1] == 1 {
				result = map[string]any{"count": 1, "list": []*znn.DetailedMomentum{momentums[call.Params[0]]}}
			}
		}
		if result == nil {
			http.Error(w, "unexpected synthetic NoM request", http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": call.ID, "result": result})
	}))
	defer nom.Close()
	control := openProtocolExecution(t, ctx, genesis, controlFixture, nom.URL)
	held := openProtocolExecution(t, ctx, genesis, heldFixture, nom.URL)

	for height := uint64(2); height <= boundary+1; height++ {
		frontier.Store(height)
		if advanced, err := control.deriver.step(ctx); err != nil || !advanced {
			t.Fatalf("supported derivation at %d: %v", height, err)
		}
		controlHead := control.deriver.Head()
		header := control.backend.BlockChain().CurrentBlock()
		if controlHead.Height != height || header.Hash() != controlHead.L2Hash || header.GasUsed == 0 {
			t.Fatal("control did not execute and accept the expected block")
		}
		before, requests, progress := held.deriver.Head(), held.requests.Load(), held.deriver.lastProgress.Load()
		advanced, err := held.deriver.step(ctx)
		stored, dbErr := held.deriver.DB.Head()
		if dbErr != nil || stored == nil {
			t.Fatalf("read held node store: %v", dbErr)
		}
		if height < boundary {
			if err != nil || !advanced || *stored != controlHead || held.deriver.Head() != controlHead ||
				held.backend.BlockChain().CurrentBlock().Hash() != controlHead.L2Hash || held.requests.Load() <= requests {
				t.Fatalf("independent prefix disagrees at %d: %v", height, err)
			}
			continue
		}
		var halt *release.ProtocolHalt
		if !errors.As(err, &halt) || halt.Requirement != requirement || *stored != before ||
			before.Height != boundary-1 || held.deriver.Head() != before || held.requests.Load() != requests ||
			held.deriver.lastProgress.Load() != progress || held.backend.BlockChain().CurrentBlock().Hash() != before.L2Hash {
			t.Fatalf("unsupported derivation changed a protected boundary at %d: %v", height, err)
		}
		status := held.deriver.Status()
		if !status.Protocol.Blocked || status.Protocol.Requirement == nil || *status.Protocol.Requirement != requirement ||
			status.Protocol.CheckedMomentum != boundary {
			t.Fatal("the original protocol hold was lost")
		}
	}
	if control.derived != 4 || held.derived != 2 {
		t.Fatalf("unexpected committed callbacks: control %d, held %d", control.derived, held.derived)
	}
	t.Logf("Embedded clients agreed through momentum %d. Control committed %d blocks. The guard held at %d after %d blocks with zero further Engine API requests.",
		boundary-1, control.derived, boundary, held.derived)

	t.Run("reopened database holds before engine access", func(t *testing.T) {
		before := control.deriver.Head()
		if err := control.deriver.DB.Close(); err != nil {
			t.Fatal(err)
		}
		db, err := store.Open(control.databasePath)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = db.Close() })
		reopened := &Deriver{Cfg: heldFixture.Config, Protocol: heldFixture.Guard, DB: db}
		err = reopened.Init(ctx)
		var halt *release.ProtocolHalt
		after, dbErr := db.Head()
		if !errors.As(err, &halt) || halt.Requirement != requirement ||
			dbErr != nil || after == nil || *after != before || !reopened.Status().Protocol.Blocked {
			t.Fatalf("unsupported persisted head was not held on reopen: %v", err)
		}
		t.Logf("Reopened SQLite retained momentum %d and rejected initialization before any execution-client access.", after.Height)
	})
}
