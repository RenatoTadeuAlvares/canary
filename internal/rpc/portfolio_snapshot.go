package rpc

import "time"

// MethodPortfolioSnapshot is the daemon-owned current valuation projection.
const MethodPortfolioSnapshot = "portfolio.snapshot"

// PortfolioAllocation preserves signed values and missing inputs; it is not risk exposure.
type PortfolioAllocation struct {
	Name       string   `json:"name"`
	ValueBase  *float64 `json:"value_base"`
	PercentNLV *float64 `json:"percent_nlv"`
	Observed   int      `json:"observed"`
	Missing    int      `json:"missing"`
}

// PortfolioSnapshotResult separates current holdings valuation from statement returns.
type PortfolioSnapshotResult struct {
	AsOf              time.Time             `json:"as_of"`
	AccountAsOf       time.Time             `json:"account_as_of"`
	PositionsAsOf     time.Time             `json:"positions_as_of"`
	Authority         *AccountDataAuthority `json:"authority"`
	BaseCurrency      string                `json:"base_currency"`
	NetLiquidation    *float64              `json:"net_liquidation"`
	AssetClasses      []PortfolioAllocation `json:"asset_classes"`
	Sectors           []PortfolioAllocation `json:"sectors"`
	SectorBasis       string                `json:"sector_basis"`
	CostBasisBase     *float64              `json:"cost_basis_base"`
	CostBasisObserved int                   `json:"cost_basis_observed"`
	CostBasisMissing  int                   `json:"cost_basis_missing"`
	CoverageStatus    string                `json:"coverage_status"`
}
