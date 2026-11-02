# Queued wallet ACK recovery in marketplace fixtures

This candidate adds separate `local-sync` profiles to the existing current-source
Relayer marketplace matrix. The default profiles remain `volatile`. Both use the
same unchanged `edgepillar/zvm-relayer` candidate at
`f5d55cce233a90fa962837671b0371004ea56a12` with Core
`29e6d9b30be94de3772c850c7a1e9811585a8833`. Embedded revision/clean-tree checks and
per-start file identity, size and SHA-256 checks remain required.

All nine existing two-transaction approval/cancellation profiles receive a
separately named durable variant: four native partial batches, the mixed-payment
partial batch, and four missing-allowance profiles. The original 43 profiles and
their 996 financial checks remain required. The proposed combined matrix is 52
profiles, 1,244 named financial checks, 70 owned provider transactions/test NoM
publications and 79 real Relayer process lifetimes. Nine new SIGKILL joins are
required. These numbers are source declarations until fresh hosted execution.

The first actual CLI uses `-max-wait 1h` and a fresh owned admission-store path,
with four retained records and 65,536 logical signed bytes as explicit limits.
Only one sub-1,024-byte transaction is submitted before the kill, so a full
16-KiB batch cannot override the declared delay. The production page and its
original EIP-1193 call must receive the actual CLI's original transaction hash.
No wallet, product HTTP or RPC reply is fabricated to create an ACK.

Before SIGKILL, actual status must show complete recovery, `local-sync`, durable
admission, one queued transaction, no posted transaction and exactly the original
signed bytes under healthy store limits. The owned source must still have zero
publications and no executed receipt. The kernel-killed CLI must be joined.
Only then does an independent read-only version-one record oracle compare the
actual queued wire bytes, phase, domain and checksum. It does not call the
production store decoder or synthesize a replacement journal.

The next process opens that same store and endpoint using the ordinary 10-ms
posting delay. Readiness still requires complete source/journal recovery. The
one original transaction may be queued or posted, but its signed bytes and
active/retained counts must remain exact. The unmodified CLI must produce one
signed NoM account block containing those same bytes, without another wallet
submission. Original source binding, application receipt, balances, allowance,
ownership, prices, refunds, payout, canonical replacement/restoration and browser
joins remain required before the next wallet action completes.

After the first transaction executes, another ordinary process reopen must
discard its active nonce while retaining the original audit record and bytes.
This is distinct from deleting or pruning retired evidence. The second actual
wallet transaction and all original financial oracles then run unchanged.

One Relayer binary and the existing unchanged frontend builds serve the entire
matrix. Journals are fresh hosted test fixtures; no operator database is opened
or migrated. Local native execution or dependency downloads are not required.
SIGKILL and selected LevelDB sync behavior do not qualify hardware power loss,
independent NoM authority/finality, production wallets, operator capacity or
retention policy, default-scale histories/checkpoints, broader platforms or
maintainer acceptance. All original 42 requirements remain active; production
is **NO_GO**.
