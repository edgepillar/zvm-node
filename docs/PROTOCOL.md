# Release-pinned unsupported protocols

When `-protocol-policy` is bound to independently accepted release inputs, an
unsupported rule at momentum H stops before planning H. A previously persisted
or engine head at H or later is quarantined before reconciliation; it is never
automatically rewound to bypass the hold. Changing the accepted normalized
configuration also holds derivation. Inspect `protocol` in `/healthz` or
`/api/info`: the process remains available, and health returns 503 immediately.

Retain the original database, engine state, accepted release inputs and block
hashes. Qualify a supported release and its independently reviewed policy before
restarting. A reorg or a lower frontier does not clear the process's latched
hold. This mechanism does not receive a NoM upgrade notice or decide a real
network activation; those require separate protocol and governance acceptance.
