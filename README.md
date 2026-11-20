# zvm-node

The ZVM's derivation node. It reads momentums from a Zenon node, turns each one
into exactly one EVM block, drives op-reth through the Engine API, indexes the
result into SQLite and serves it over a read API.

```sh
git clone --recurse-submodules https://github.com/Zenoglyphs-VM/zvm-node
go build ./...
go test ./...
docker build -t zvm-node .

zvm-node -config l2/rollup.json \
         -l1 http://127.0.0.1:45997 \
         -engine http://127.0.0.1:8551 -eth http://127.0.0.1:8545 \
         -db /data/zvm.db -listen :8080
```

| | |
|---|---|
| `GET /healthz` | 200 while keeping up, 503 on a protocol hold or stalled derivation |
| `GET /api/info` | chain parameters, fork status, derivation head |
| `/api/...` | blocks, transactions, tokens, momentums, contracts, prices, NFTs |
| `POST /rpc`, `POST /l1rpc` | proxied to op-reth and to NoM |

Derivation retries forever rather than exiting, so watch `/healthz`, not the
process. On a public hostname, set `-rpc-deny debug_,txpool_,admin_`.

Subcommands: `bench`, `apps`, `media`, `solcd`, `dex`, `verify-tokens`
(`-h` on each). Their endpoints default to `NOM_NODE`, `ZVM_RPC`, `ZVM_NODE`
and `ZVM_RELAYER`; `.env.example` has a local zvm-devnet's. No node is built
in. A whole local devnet is in
[zvm-fleet](https://github.com/Zenoglyphs-VM/zvm-fleet)'s `devnet/`.

Consensus lives in [zvm-core](https://github.com/Zenoglyphs-VM/zvm-core), a
submodule here.

MIT licence; GPL-3.0 files are listed in [NOTICE.md](NOTICE.md). The full
guide, with how it works and why: [docs/GUIDE.md](docs/GUIDE.md).

## Signed network declaration

`GET /api/release` returns an owner-signed release assertion when all four
startup inputs are configured: `-release-manifest`, `-release-policy`,
`-release-signatures` and `-release-trusted-keys`. They must be configured together.
The node verifies their policy and signature quorum and binds the exact rollup
configuration bytes before reading its JWT, opening a database or dialing a
service. A mismatch refuses startup. The response is frozen when the API handler
is constructed and uses `Cache-Control: no-store`; extension `/api/info` fields
do not determine this declaration. An unconfigured node returns 503 from this
route and can still serve read-only clients.

Applications must provision their own public keys and signature quorum. Neither
this route nor its host is a trust root. The assertion explicitly retains
`runtimeObserved: false`: this startup check does not verify the complete offline
release input set, running images, live contract code or L1 consensus. Perform
`zvm release-verify` and independent runtime qualification separately. Rotate the
signed epoch ID on every reset, even if the chain IDs and genesis are reused.
An undeclared reset cannot be detected by a static signed declaration alone.

## Protocol activation holds

An independently accepted release can pin an additional `-protocol-policy`
document. Its signed manifest file ID must be `protocol-activation-policy`,
with kind `config`, and the independent inventory must require that entry.
All four release flags are required. Missing, modified, oversized or wrongly
bound policy bytes refuse startup before JWT access, database opening or service
dials. A release that declares the policy must supply the flag. A legacy release
without that entry remains usable and reports `protocol.enforced: false`.

Engine compatibility and activation authority each require independent review.
The schema and compiled rule capabilities are documented with the Core guard.
[Core protocol guard](https://github.com/Zenoglyphs-VM/zvm-core/pull/5)
The accepted policy is bound to the epoch and exact rollup bytes, and cannot
omit or move an enabled rule's configured first activation. Future unknown rules
or higher versions cause a hold at their activation momentum.

The node checks before planning a block or contacting the engine for that block.
An unsupported persisted or engine head is held before reconciliation can rewind
it. Failed boundaries preserve canonical heads, progress markers and derived
message outcomes. The index follower starts only after this initialization check.
The process stays available for diagnosis; `/healthz` immediately returns 503
without waiting for the stall timer. Both it and `/api/info` include `protocol`
with enforcement, capabilities, pinned requirements and the held reason. Info
extensions cannot replace this field. A hold is latched until restart with a
separately accepted release; lowering a head or temporarily restoring mutated
configuration does not clear it.

The protocol rehearsal feeds the Node derivation loop from a synthetic NoM
sequence. Two embedded op-geth instances and SQLite stores check agreement
before the boundary, zero Engine API requests from the held node at and after
the boundary, and an unchanged database head on reopen. Real activations
require a separately agreed schedule, authenticated NoM notices, distinct
release and client rehearsals, and operator acceptance.
