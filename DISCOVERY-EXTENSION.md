# Controlled read-only contract discovery extension

This temporary Investor fork extends `68e6d89c6788feae57f908870496ea186d50b289`
with `Connector.DiscoverContracts(ctx, pattern)`. It reuses the existing
connection, framing, pacing, request IDs and socket epoch guard. It sends
`reqMatchingSymbols` and positive-ConID `reqContractDetails` only. Each details
request uses the returned candidate's explicit currency and listing venue;
neither SMART nor USD is manufactured. All candidates are retained.

The result requires `symbolSamples` and positive `contractDetailsEnd` receipts
under the captured session. Missing/invalid fields, truncated details, timeout,
or a changed/disconnected session return incomplete evidence. It does not
subscribe to quotes, place/cancel orders, reconcile an account, or grant trading
admission. The caller must resolve semantic ambiguity among returned contracts.
ISIN is decoded from the returned security-identifier list; absent ISIN stays
absent. Full public contract callback fields are retained for evidence.

The API is additive. Existing contract classification behavior is unchanged;
the stricter ISIN conflict check applies only to this discovery result.

Protocol reference: [IBKR Stock Contract Search](https://interactivebrokers.github.io/tws-api/matching_symbols.html).
The extension remains controlled and temporary pending possible upstream
contribution; no second TWS implementation or alternate transport was added.

Verification includes `TestDiscovery*` (including race detection) and existing
contract parser regression tests. The full broker package currently has an
unrelated option-cache restart fixture failure: its fixed option expiry is
2026-08-21. The unchanged admitted revision also fails that test. Its existing
position-disconnect test fixture also writes connection status without its lock
and fails race detection. Neither unrelated fixture is weakened by this change.
