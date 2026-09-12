# Continuous display stream

`canary market --watch --json` opens the read-only `display.subscribe` Unix RPC
and prints newline-delimited JSON. This stream owns presentation only. It does
not grant broker-write authority, run a model, persist decisions, or re-evaluate
risk. Unary account, portfolio, market and historical tools retain their existing
contracts. TWS account values and P&L have their own source cadences.

## Wire contract

Version 1 is a complete replacement snapshot, bounded to 512 KiB. Every RPC
connection starts a random generation and monotonically increasing sequence.
Scope contains the proven account ID and mode. On loss of socket/backend/scope
or portfolio readiness the RPC ends; consumers retain previous data only as
unavailable saved context. A reconnect starts a new generation. There are at most
four concurrent display clients and 128 requested quote instruments per client.
`truncated` includes incomplete subscription acquisition; absent rows/fields
remove the previous current overlay rather than silently retaining old values.

- `emitted_at` is transport time, never a source freshness clock.
- Account NLV uses a dedicated account-attributed streaming cache. Its clock is
  the accepted NLV receipt. Base currency must be proven by broker fields.
- Account P&L and its receive clock are validated against the subscribed account.
- Positions retain exact contract identity, quantity, cost and broker monetary
  values. `positions_at` is the portfolio heartbeat; `valuation_at` is the
  individual portfolio receipt. `unrealized_at` identifies the actual unrealized
  P&L receipt. P&L stream values supersede portfolio P&L only when units match.
- Position `daily_pnl` is supplied only when contract and proven account base
  currencies match. The legacy per-contract P&L FX interpretation is unresolved;
  this stream performs no speculative conversion.
- Quotes include exact routed contracts, data type, price selection, bid/ask,
  previous close, broker trade time, independent local price-receive time and
  cumulative volume with its own receipt time. A bid, close or volume receipt
  cannot advance last-price freshness. Zero volume is observed, not missing.
- Historical TRADES bars and their linked volume remain a separate request
  contract. Neither historical bars nor regular close are synthesized from ticks.

## Owners and failure policy

The connector owns its account/portfolio/P&L subscriptions. A nonblocking
post-dispatch invalidation wakes display readers. Account/portfolio/P&L copies
share the existing publication/inbound/evidence lock order. That boundary is
released before copying quote caches, because subscription operations may hold
the quote-cache mutex while waiting for broker I/O. A final exact-session check
rejects a reconnect-crossing capture. Separate field clocks describe different
receipt instants; the snapshot is not a simultaneous valuation assertion.

The daemon shares quote holds with routed unary snapshots through subManager.
Each entry and releaser are bound to their originating connector/session;
retired releases and errors cannot remove a replacement entry. The display
worker handles acquisition away from publication, checks superseding universes
between acquisitions, retries missing subscriptions every 30 seconds, and
re-resolves front futures daily. Its P&L requests honor cancellation and the
existing 50-contract ceiling. This is not an entitlement guarantee.

Each display handler owns its client EOF watcher and acquisition worker. Writes
have a five-second deadline. Cancellation closes the client socket, stops work,
releases all holds with one shared three-second cleanup deadline, then joins
workers. Server.Stop rejects new display registrations, cancels and joins active
ones before closing shared subscriptions, the connector or durable stores.
Panic containment preserves the daemon failure policy. Idle caches are not
polled for display; source invalidation is coalesced for 100 ms. The 30-second
retry is subscription maintenance, not price acquisition.

## Integration

Torok owns a single volatile feed producer per configured service, bounded
latest-state fanout, reconnect backoff and shutdown join. Desk supplies this CLI
adapter and multiplexes market frames onto its authenticated same-origin SSE.
Slow readers receive complete newest state. Display updates do not create
SQLite writes or agent work. The slower source observers still retain decision
context. The broker owns inventory; Desk does not calculate a second ledger.

## Verification contract

Primary tests cover source attribution/clocks, zero and invalid volume, currency
withholding, timestamp dispatch, last-reference teardown, retired-session
cleanup, and daemon shutdown join. Desk tests cover CLI input bounds/scope,
child reaping, authenticated SSE without snapshot sampling and browser overlays.
A deliberate Torok retired-producer regression must fail its lifecycle test.
Run `make modernize`, `make docs-regen` when the command catalog changes, and
`make test` before integration. Use only redacted `status` and stream schema/
coverage receipts for runtime reports; do not publish account data.

Rollback removes the display RPC/CLI, notification/projection and shared-hold
changes together, restores Desk's previous Torok pin and display adapter, then
rebuilds/restarts both managed services. No persistent stream data migration,
trading policy change or broker order is involved.
