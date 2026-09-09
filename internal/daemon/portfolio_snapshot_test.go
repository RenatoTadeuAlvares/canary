package daemon

import (
	"github.com/osauer/canary/v2/internal/rpc"
	"testing"
)

func TestPortfolioCannotMultiplyOptionCostTwiceOrHidePartialValues(t *testing.T) {
	n := func(v float64) *float64 { return &v }
	a := &rpc.AccountResult{BaseCurrency: "USD", NetLiquidation: 1000, TotalCash: 1200, Authority: &rpc.AccountDataAuthority{Fields: &rpc.AccountFieldAvailability{NetLiquidation: true, TotalCash: true}}}
	p := &rpc.PositionsResult{Options: []rpc.PositionView{{ConID: 11, Symbol: "SYNTH", SecType: "OPTION", Currency: "USD", Quantity: -2, AvgCost: 100, Multiplier: 100, MarketValueBase: n(-150)}}}
	r := projectPortfolio(a, p, map[string]string{"SYNTH": "Technology"})
	if r.CostBasisBase == nil || *r.CostBasisBase != -200 {
		t.Fatal("option cost multiplied twice")
	}
	if r.CoverageStatus != "complete" {
		t.Fatal("observed synthetic inputs incomplete")
	}
	p.Options = append(p.Options, rpc.PositionView{ConID: 12, Symbol: "GAP", SecType: "OPTION", Currency: "EUR", Quantity: 1})
	r = projectPortfolio(a, p, nil)
	if r.CoverageStatus != "partial" || r.CostBasisBase != nil {
		t.Fatal("missing FX/cost/classification became complete")
	}
	for _, x := range r.AssetClasses {
		if x.Name == "Options" && (x.Missing != 1 || x.PercentNLV != nil || x.ValueBase == nil || *x.ValueBase != -150) {
			t.Fatal("partial category became full total")
		}
	}
	a.Authority.Fields.TotalCash = false
	a.Authority.Fields.NetLiquidation = false
	r = projectPortfolio(a, p, nil)
	if r.NetLiquidation != nil {
		t.Fatal("unobserved denominator became zero")
	}
	for _, x := range r.AssetClasses {
		if x.Name == "Cash" && x.ValueBase != nil {
			t.Fatal("unobserved cash became zero")
		}
	}
}
func TestPortfolioCannotAssignClassificationAcrossListings(t *testing.T) {
	p := &rpc.PositionsResult{Stocks: []rpc.PositionView{{ConID: 1, Symbol: "SYNTH", SecType: "STOCK", Currency: "USD"}, {ConID: 2, Symbol: "SYNTH", SecType: "STOCK", Currency: "EUR"}}}
	if portfolioClassificationUnambiguous("SYNTH", p) {
		t.Fatal("ambiguous listing classification accepted")
	}
}
