# Stress account identity for Desk

Stress now carries `account_scope` only when its account and positions inputs have
available, current authority and the same concrete account ID and paper/live mode.
Otherwise it omits the scope and supplies `account_scope_issue`. Missing authority,
aggregate `All`, unresolved mode and conflicting scopes cannot establish identity.

This is additive provenance on the existing `StressResult`, produced in
`internal/stress.computeStress` and shared by the existing RPC, CLI, MCP and app
read paths. It creates no new tool, risk evaluator, execution permission or policy
threshold. Existing advisory signals retain their existing calculation; consumers
must inspect scope as well as input health, source clocks and field coverage.

Stress fingerprints summarize risk categories. They deliberately omit account
identity and must never substitute for `account_scope`. The constituent clocks
remain in `source_as_of`; the response's `as_of` is not a refresh of those inputs.
Old saved stress records have no scope and remain unbound context for new consumers.

Related contracts that Desk must preserve:

- `OrdersOpenResult` uses top-level `account` and `mode`, and describes a local
  journal rather than a complete broker statement.
- `AccountResult.authority.fields` distinguishes an observed zero from an absent
  legacy numeric field. Currency availability matters for money fields as well.
- `StressPortfolioSummary.net_delta_pct_nlv` is the **absolute magnitude** of net
  dollar delta divided by net liquidation. Signed direction is available through
  `PositionsPortfolio.dollar_delta_base`.
- `greeks_coverage` counts option legs with any Greek. It does not prove complete
  directional measurement. `exposure_unmeasured` concerns base market valuation;
  it does not independently prove complete dollar delta or fresh Greeks.

The scope regression exercises absent, stale, aggregate, conflicting and matching
authority with synthetic inputs. No installed daemon, broker state, build flag,
freeze or trading setting is changed by this source work.
