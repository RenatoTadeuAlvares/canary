# Regime measurement and interpretation review

Date: 2026-09-05. Local implementation; no release or broker action implied.

## Scope and authority

The user requested an end-to-end improvement of Regime, with special attention
to gamma math and options positioning. The concurrent Edge task owns its own
retrospective review changes. Shared adapter hunks are coordinated explicitly.

This task applies `.agents/docs/daemon-cli-trading-contract.md` and the SPA
authority matrix. The daemon owns source dates, calculations and interpretation;
RPC owns typed contracts. CLI, MCP and the app format that evidence. The pure
risk policy, numeric thresholds, freeze, limits, broker authorization, submission
and protection policies are unchanged. Added skew is descriptive context and
never votes in a risk band or authorizes an action.

| Concept | Authority | Shared output | Unavailable behavior |
| --- | --- | --- | --- |
| Gamma local sign/crossings | daemon scenario model | `GammaProfileMetrics`, `GammaInsight` | No measurement stays unavailable; balanced signed/nonzero gross stays measured |
| Options pricing asymmetry | eligible same-class/expiry IV brackets | `OptionSkew`, selected skew and explanation | No extrapolation or substituted closing IV; missing sides named |
| Breadth | constituent engine and current completed session | current/history coverage and nullable secondary fields | Missing 200-session/annual coverage is not zero |
| Funding spread | date-aligned official CP/T-bill publications | Regime funding row | Later Treasury prints cannot shift only the current endpoint |
| HYG/SPY baseline | last completed daily history plus qualified quote | Regime status, source date and eligibility | Live quote cannot disguise stale historical baseline |
| Human/agent surfaces | daemon Brief and compact monitor | CLI `brief`, MCP `canary_brief`, HTTP snapshot/bootstrap/SSE and SPA | Retained context keeps source quality and last-known authority labels |

## Confirmed defects and repairs

Published v3.3.2 was checked in a disposable archive with synthetic inputs.
Two deterministic witnesses failed there and pass with this implementation:

- #38: a constituent extreme 226 sessions earlier was forgotten because only
  200 closes survived. Retaining the current plus preceding 252 closes repairs
  annual extremes and same-day corrections. Cold history now asks for 400
  calendar days, rounded by the broker to 2 Y; warm history spans downtime.
- #39: a two-minute put and seven-day call produced negative GEX at spot while
  every point of the 60-node sweep was positive. The new sweep includes exact
  spot, strikes, narrow expiry kernels and bisection of sign changes. It reports
  detected crossings and chooses the nearest; local sign determines the band
  outside the existing transition distance, including reversed crossings.

Additional corrections preserve the same measurement contract:

- Separate SPX and SPXW fits on the same expiry date. Scenario IV keeps each
  observed anchor and applies the relative change in a positive fitted curve.
  Invalid curves use a consistent sticky-IV fallback across the entire sweep.
- Preserve genuine crossings while excluding same-sign zero touches and
  far-tail floating-point zeros. Signed zero with gross exposure remains a
  balanced reading rather than missing data.
- Compare eligible observed OTM IVs at 25-delta with interpolation only between
  bracketing observations. Select a covered 7–60 day expiry nearest 30 days;
  disclose its actual tenor. Missing OI does not prevent a price-only skew read.
- Preserve separate horizon signs, gross shares and missing buckets. Agreement
  between only two covered horizons is explicitly partial.
- Include each breadth denominator in current, historical and compact results.
  Reject invalid/nonpositive/nonfinite or unordered constituent batches before
  merge, retaining the prior valid window without compressing sessions.
- Join both funding endpoints to the commercial-paper publication date with a
  maximum three-calendar-day Treasury lookback. Require current historical
  HYG/SPY baselines before confirmation. Require 252 observations for the SPY
  annual fallback.
- Allow Treasury's two concurrent monthly XML requests 25 seconds each inside
  a 30-second series budget. A live read returned valid current observations in
  18.52 seconds, while the former 10-second HTTP/12-second caller budgets
  rejected it. Other feeds and the 45-second overall refresh deadline retain
  their existing bounds; publication freshness and funding bands are unchanged.
  Select months from day one: subtracting a month from March 31 otherwise
  normalizes into March again and silently drops the previous month's history.
- Preserve served daemon bands/hysteresis in renderers and scheduled stale
  gamma context in Stress. Brief names SPX or a degraded SPY proxy and carries
  quality, feed, compute time and separate spot-observation time.

## Interpretation limits

Open interest does not identify the owners, trade direction or net intraday
inventory. Calls-positive/puts-negative is an assumption. Long modeled gamma
suggests conditional damping of either direction, short gamma amplification of
either direction. Put-minus-call IV describes relative insurance pricing, not a
bullish/bearish probability or an empirically calibrated return forecast.

The model still uses Black–Scholes zero-rate/zero-dividend approximations,
sampled chains and asynchronous observations. `top_strikes` is individual
contract concentration, not an aggregated strike wall. OI carry and existing
rankability thresholds remain unchanged. Narrow-grid refinement reduces a
known sampling failure; it is not a mathematical guarantee of discovering
every root of an arbitrary option mixture.

Method v4 invalidates old gamma current state. Breadth snapshot/window/history
v3 similarly requires rebuilt evidence. Immutable historical observations are
retained. Off-hours missing models wait for eligible feeds; they do not become
current because a software update succeeded. Retained gamma payloads include
fit measurements; the separate ranked calibration stream is disabled with
production storage, so no calibrated predictive improvement is claimed.

Primary interpretation sources: Cboe's 2023 SPX 0DTE market-impact analysis and
its Option Sentiment specifications, linked in the public Gamma concepts page.

## Evidence

Private before/after JSON and logs remain in `/tmp/canary-regime-review`.
Completion reports use only command status, schema/fingerprint and selected
source-health fields, never private account data.

Focused tests cover narrow expiry pockets, nearest genuine crossings, invalid
smile fallback, class isolation, observed-IV anchoring, balanced exposure,
bracketed skew and missing sources, local-sign depth, breadth history/catch-up,
invalid batch retention, funding alignment and bounded Treasury latency,
stale-baseline eligibility,
CLI/MCP parity, and source-preserving compact projections.

The production SPA's synthetic mobile browser fixture exercises horizon
disagreement, unavailable buckets, frozen context, richer downside pricing and
safe text rendering. It accesses no desk account and is separate from physical
phone pairing or installability proof.

The combined product commit `0474b1651bd800e3706e2e1d228df1490f27358b`,
including the concurrent Edge changes, passed `make test` (including `check`,
race tests, both daemon build modes and all four regression-spine mutants).
It was installed from a clean detached checkout. The first restart encountered
a new daemon already started by the existing app; version, executable and
Gateway readiness verified the intended clean build, and the repeated
`make restart-daemon` correctly skipped a redundant restart.

Fresh installed CLI and MCP reads preserve the same gamma/breadth measurements
(excluding per-read clocks). The direct Regime RPC reports healthy funding,
aligned CP/T-bill dates and a populated five-publication change. Gamma is cold
under the new method while options are off-session; breadth is rebuilding its
longer history. Regime correctly remains in data quality rather than reporting
a complete fresh signal.

`make app-smoke-read-only` on the isolated `127.0.0.1:8766` preview verifies
exact installed provenance, listener ownership, source-matching assets, the
server's explicit read-only grant, same-origin reads, and zero pairing/credential
writes. It checks the visible app surface and the read-only marker in its status
text; the existing status text is visually hidden, so this proves no visible
read-only badge.
The driver was corrected to accept the explicit preview grant as well as the
normal unpaired screen; normal authenticated sessions and non-read requests
remain rejected. Its follow-up changes affect only the test driver and this
evidence note, with `make check` as the pre-commit gate. The installed product
and served assets remain the tested `0474b165` build. The shared phone app host
was left untouched. No release is part of this task.
