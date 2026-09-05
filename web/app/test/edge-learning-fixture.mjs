// Synthetic UI evidence. These values exercise presentation, not the engine's
// arithmetic (which has independent hand-calculated Go fixtures).
export function withEdgeLearning(result) {
  result.review_action = "add";
  result.review_direction = "long";
  result.review_note = "The covered decisions span 1 month and 1 execution date. This is a price outcome, not proof of skill or risk-management quality.";
  result.protection_state = "current";
  result.protection_as_of = result.as_of;
  result.patterns = result.action_rollups.flatMap((row) => {
    const eligible = Math.max(...row.horizons.map((h) => h.sample_count));
    if (!eligible) return [];
    return [{
      action: row.action, direction: "long", eligible_changes: eligible, notional_known_count: eligible, known_notional_base: eligible * 1000,
      horizons: [1, 5, 20].map((sessions) => {
        const h = row.horizons.find((item) => item.sessions === sessions) || { sessions, sample_count: 0 };
        const n = h.sample_count;
        return {
          sessions, sample_count: n, linked_protection_count: 0, partial_protection_count: 0,
          scored_notional_base: n ? n * 1000 : undefined, notional_coverage_pct: n ? n / eligible * 100 : undefined,
          total_base: h.total_base, median_base: h.median_base, median_impact_pct: n ? 1 : undefined,
          positive_count: n && h.total_base > 0 ? n : 0, negative_count: n && h.total_base < 0 ? n : 0, flat_count: n && h.total_base === 0 ? n : 0,
          distinct_dates: n ? 1 : 0, distinct_contracts: n ? 1 : 0,
          largest_date_share_pct: n ? 100 : undefined, largest_contract_share_pct: n ? 100 : undefined, without_largest_base: n ? 0 : undefined,
          months: n ? [{ month: "2026-01", sample_count: n, total_base: h.total_base, median_base: h.median_base }] : [],
          exclusions: eligible > n ? { intervening_change: eligible - n } : {}, market_context: [],
        };
      }),
      comparisons: [5, 20].map((later) => {
        const first = row.horizons.find((h) => h.sessions === 1);
        const last = row.horizons.find((h) => h.sessions === later);
        const matched = first?.sample_count > 0 && first.sample_count === last?.sample_count;
        return { earlier_sessions: 1, later_sessions: later, sample_count: matched ? first.sample_count : 0,
          earlier_total_base: matched ? first.total_base : undefined, later_total_base: matched ? last.total_base : undefined,
          difference_base: matched ? last.total_base - first.total_base : undefined, median_difference_base: matched ? (last.total_base - first.total_base) / first.sample_count : undefined };
      }),
    }];
  });
  result.options.cycles = {
    completed_count: 1, complete_pnl_count: 0, open_contract_count: 0, excluded_contracts: 1, reasons: { lifecycle_or_transfer: 1 }, known_pnl_base: 90, truncated: false,
    cycles: [{ id: "option-cycle_synthetic", symbol: "SYN CALL", direction: "long", opened_at: "2026-01-05T12:00:00Z", closed_at: "2026-01-20T12:00:00Z", execution_count: 3, realized_pnl_base: 90, pnl_status: "partial", missing_evidence: ["realized_pnl"], linked_protection_count: 0 }],
  };
  return result;
}
