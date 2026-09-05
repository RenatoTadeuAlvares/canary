## 1. Summary

Reviewed the Edge working changes over 7c578560: Go analysis, daemon, RPC, CLI/MCP,
JavaScript SPA and supporting documentation/tests. Two independent read-only
lenses covered architecture/Go/dead weight and supporting surfaces/readability.
The design is useful with explicit evidence bounds. Five medium findings were
verified and addressed in this task; no high finding was raised. Focused Edge
checks and synthetic narrow-screen render passed after the changes. Full shared
tree validation and installed-runtime acceptance are recorded in the task's final
completion evidence; the initial full gate stopped on modernization (fixed), then
concurrent Regime test/formatting work. No broker writes were exercised.

HIGHs claimed: 0 → kept: 0, downgraded: 0, dropped: 0.

| Area | HIGH | MED | LOW |
| --- | ---: | ---: | ---: |
| Overall, before fixes | 0 | 5 | 0 |
| Core and daemon | 0 | 3 | 0 |
| Supporting surfaces | 0 | 2 | 0 |
| Unresolved release blockers | 0 | 0 | 0 |

## Pre-flight context
- Snapshot: 7c578560; Edge working changes plus separate concurrent Regime and user Codex config changes.
- Scope: This task's Canary Edge additions only; exclude concurrent Regime/config work. Daemon owns publication and provenance, edge pure analysis, rpc typed contracts, adapters presentation. Read-only review.
- Repo shape: Go daemon/analysis/RPC/CLI/MCP, JavaScript SPA and browser test, Markdown docs. 2023 Go files total. Largest relevant: analyze.go 1642, edge_authority.go 1352, edge_authority_test.go 1029, rpc/edge.go 907, web/app/edge.js 896, analyze_test.go 892, cli/edge.go 458, model.go 409, edge_flex_acquisition.go 342, edge_acceptance_test.go 274, rpc/edge_test.go 226, learning.go 225, option_cycles.go 221, rpc/edge_learning.go 215, cli/edge_test.go 191 lines.
- Native verification: focused Edge Go/CLI/MCP/RPC tests, app-check and synthetic app-render-check pass. Full make test (check, race, render, historical regression witnesses), docs regeneration and installed live QA pending. No live orders.
- Deliberately-kept surfaces:
  - Realized option episodes and separate open snapshot — kept per 51baa389: distinct overlapping financial scopes, no combined P/L; cycles are an additional proven subset, not replacement accounting.
  - Public typed RPC vocabulary — d63d818f explicitly removed unproducible values while retaining actual cross-surface contracts.
- Recent subtraction passes:
  - 7404bae9 — removed smoke telemetry wrappers, kept risk-triggered native smoke targets.
  - d63d818f — removed unproducible wire values and unreachable display branches.

## 3. Findings table

All rows below describe the reviewed state and record the applied disposition.
The base commit remained unchanged during review; Edge fixes and concurrent
Regime edits changed the worktree. Cited corrections were rechecked afterward.

| ID | Severity | Location | Claim and disposition |
| --- | --- | --- | --- |
| F-01 | MED | internal/daemon/edge_protection_context.go:34 | Unbounded proposal reload on every read; replaced by a bounded, completeness-checked read. |
| F-02 | MED | internal/daemon/edge_authority.go:1044 | Competing aggregate and direction-specific headline selection; aggregate selector removed. |
| F-03 | MED | internal/edge/analyze.go:124 | Repeated trade grouping and anchor preparation; reused the existing inputs before replay. |
| F-04 | MED | web/app/edge.js:825 | Monthly evidence absent from the visible workflow; added expandable app detail and CLI rows. |
| F-05 | MED | web/app/edge.js:864 | Partial cycle P/L looked complete when collapsed; included evidence status in the title. |

## 4. Findings detail

### Core and daemon

**F-01 — Bound optional context reads.**
Why: Complete proposal-history pagination made a snapshot read grow without a
bound. Evidence: `loadProposalEvents` called `loadAllCoreEvents`; that loop in
`daemon_state_sqlite.go` paginates until exhausted. Action taken: Edge directly
loads at most 10,000 proposal records, and withholds context at the cap, matching
the existing bounded order read. Financial evidence remains available. A future
scoped revision could reduce this fixed maximum cost; it is not needed for
correctness. Account/mode filtering follows retrieval, so a large sibling-account
history can conservatively suppress optional context. This limitation is recorded
in the design contract rather than silently treating a prefix as complete.

**F-02 — Select one review population.**
Why: A pooled long/short fallback contradicted the new direction-specific sample.
Evidence: two long and two short opens cleared the aggregate count of three but
neither direction qualified. Action taken: removed both old aggregate selectors;
headline, market context and note use the single selected direction-specific
population. `TestEdgeHeadlineDoesNotPoolOpposingDirections` covers the counterexample.

**F-03 — Reuse reconciliation preparation.**
Why: The cycle addition repeated grouping, mutations, scoring-index setup and
quantity-anchor reconciliation already performed by Analyze. Evidence: the old
cycle entry repeated Analyze's preparation verbatim. Action taken: cycle analysis
receives initial quantities and unbalanced-contract evidence before the stock
replay mutates quantities. It still processes raw option executions chronologically.
The option-cycle boundary suite passes after this change.

### Supporting surfaces

**F-04 — Show the monthly evidence.**
Why: Month sign counts alone did not let users inspect sample size or magnitude.
Evidence: the daemon returned monthly counts, totals and medians, but the original
app and CLI did not render them. Action taken: added collapsed monthly details
for the selected group and equivalent CLI rows. The production SPA behavior test
asserts the new detail; money still uses the existing privacy masking helper.

**F-05 — Label partial cycle amounts immediately.**
Why: A known subtotal and complete P/L had identical collapsed titles. Evidence:
the cycle engine deliberately produces a numeric subtotal with `partial` status;
the original title omitted that status. Action taken: reused the existing option
evidence label in each title. The synthetic fixture now supplies partial P/L and
the behavior test verifies its collapsed label.

## 5. Implementation plan

### 1. Unify selection and reuse evidence
- Files: internal/daemon/edge_authority.go, internal/daemon/edge_learning.go, internal/edge/analyze.go, internal/edge/option_cycles.go.
- Bundles: F-02, F-03. Effort: S. Status: completed.
- Why first: Removes contradictory selection and duplicate preparation.
- Acceptance: Focused direction-separation and option-cycle suites pass.
- Risks: Account materiality and existing option accounting must remain unchanged.

### 2. Bound local provenance work
- Files: internal/daemon/edge_protection_context.go, internal/daemon/edge_protection_context_test.go, internal-docs/design/edge-learning-review.md.
- Bundles: F-01. Effort: S. Status: completed.
- Why first: Keeps optional history work bounded without weakening completeness.
- Acceptance: Changed/unavailable provenance is withheld across every surface while P/L is invariant; cap handling is explicit in source and docs.
- Risks: Large histories can lose optional context; a prefix must never be accepted as complete.

### 3. Expose inspection details
- Files: web/app/edge.js, internal/cli/edge.go, web/app/test/edge-learning-fixture.mjs, web/app/test/production-behavior.test.mjs.
- Bundles: F-04, F-05. Effort: S. Status: completed.
- Why first: Lets users inspect repeatability and distinguish partial P/L.
- Acceptance: Production behavior assertions and narrow-screen synthetic rendering pass.
- Risks: Financial values must remain hidden when privacy mode is active.

novel HIGH this round: 0 · HIGHs verified: 0/0 · iterations: 1
