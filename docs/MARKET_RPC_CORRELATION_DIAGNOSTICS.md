# Owned fixture RPC correlation

The executed-market fixture adds one random, local-only header to intercepted
same-origin POST /rpc requests. The SDK-supplied header view is copied once;
the request URL, method and body remain unchanged. Missing, invalid, reserved
or bounded-out observations use the original continuation call. Synchronous
throws and promise rejections propagate once. This changes the fixture's HTTP
header arguments; the separate method-outcome observer remains unchanged.

A separate server ledger echoes the owned header and tracks at most 256
records and 32 active handlers. Duplicate nonces are ambiguous. It preserves
the original writer, request, status, body and panic, and releases completed
contexts. The original aggregate progress observer is unchanged. A capability
protected read-only fixture endpoint selects at most eight records in one
bounded query; it reports positions and lifetime booleans, without nonces,
headers, request bodies, URLs, capabilities or error text.

The passive navigation hook reads only the owned response header from existing
Network response-extra-info events. It supports an echo preceding the matching
request event or arriving after its completion event. Four primary sessions
retain at most 256 protocol identities and 256 early echoes each. Session scopes
are isolated; reused identities remain ambiguous instead of being forgotten.
Conflicts, unknown tags and observations exceeding the bound are refused. Opaque selections retain
nonces privately. A matching local tag, primary-session echo and unique server
record establish only an owned HTTP transaction correlation. Header receipt,
normal handler return and continuation fulfillment prove neither response-body
delivery nor protocol-session identity, authentication or navigation success.

One optional query runs before the original cleanup with a 750 ms deadline,
16 KiB body bound and eight-target limit. It reports a pending request when a
unique echo is available, otherwise a labeled recent sample or unavailability.
It adds no product RPC, provider transaction or protocol command. Diagnostics
cannot complete a request or weaken the original 20-second deadline, 100 ms
quiet period, zero-active rule, signature, receipt, price, refund, payout,
balance or same-test-fragment oracle. Current failure causes remain
NOT_ESTABLISHED until separately supported by actual source-bound evidence.

The additional Go controls run inside the existing normal/race-selected
TestMarketRPCProgress parent, retaining every original named subtest. Local
standard-library controls use fake requests/events/fetch and no SDK, browser or
real network. Runtime correlation and the complete marketplace matrix require
fresh qualification at the candidate source. All original roadmap gates remain.
