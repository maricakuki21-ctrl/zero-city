# Billing contract v1

- API decimals are JSON strings; JSON numbers are rejected.
- Persistent decimals use PostgreSQL `NUMERIC(24,12)` and never pass through `float64`.
- `0.0001` round-trips canonically as `"0.0001"`.
- Accepted quote snapshots are copied, SHA-256 verified, versioned and immutable.
- A hold copies the quote SHA, exact amount/asset, original expiry and source epoch. Control state, lease and version are separate.
- State and accepted-media changes require the current active epoch. Effective mutations additionally require the expected version and increment it once.
- Capture/release replay of the same terminal result and media-task replay of the same ID are no-ops; conflicting terminal results or media IDs fail closed.
- Rounding uses half-even only when a caller selects a posting scale. The signed residue is returned explicitly and must become a balanced journal entry; it is never discarded.
- Repository, migration and gateway adapters are intentionally deferred. They must persist these exact strings and implement CAS with `WHERE version = $expected AND lease_epoch = $active_epoch`.
