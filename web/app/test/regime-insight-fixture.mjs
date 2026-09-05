// Public-symbol synthetic observations for rendered Regime coverage.
export function withRegimeInsights(snapshot, asOf) {
  snapshot.regime = {
    ...snapshot.regime,
    authority_health: { status: "current", last_success_at: asOf },
    gamma_insights: [{
      underlying: "SPX", as_of: asOf, data_type: "frozen", rankability: "context_only",
      rankability_reason: "Closed-session observation; confirm after the next eligible refresh.",
      regime: "long_gamma",
      interpretation: "Modeled hedging would dampen moves in either direction. 2 crossings detected; the displayed level is nearest spot.",
      positioning_assumption: "OI model: calls positive, puts negative; gross gamma is sampled convexity, not observed dealer hedges.",
      directional_inference: "Bullish/bearish positioning is unknown: open interest does not identify owners or trade direction.",
      skew_interpretation: "SPXW 20261002 (27.0 days): 25-delta put minus call IV +3.00 vol points — richer downside protection. This is option pricing, not a directional forecast.",
      horizons: [
        { horizon: "0dte", regime: "short_gamma", gross_share_pct: 20 },
        { horizon: "1to7", regime: "unavailable" },
        { horizon: "term", regime: "long_gamma", gross_share_pct: 80 },
      ],
      horizon_agreement: "diverge:0dte_vs_term",
    }],
  };
  return snapshot;
}
