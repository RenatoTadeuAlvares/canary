package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/osauer/canary/v2/internal/rpc"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

func currentPortfolioAuthority(a *rpc.AccountDataAuthority) bool {
	return a != nil && a.Availability == rpc.AccountDataAvailable && a.Freshness == rpc.AccountDataFreshnessCurrent && a.Scope.AccountID != "" && a.Scope.AccountMode != ""
}
func (s *Server) handlePortfolioSnapshot(ctx context.Context) (*rpc.PortfolioSnapshotResult, error) {
	c := s.gatewayConnector()
	if c == nil {
		return nil, s.gatewayUnavailableError()
	}
	binding, ok := c.CaptureSession()
	if !ok {
		return nil, s.gatewayUnavailableError()
	}
	p, err := s.handlePositionsList(ctx, &rpc.Request{Params: json.RawMessage(`{}`)})
	if err != nil {
		return nil, err
	}
	a, err := s.handleAccountSummary(ctx)
	if err != nil {
		return nil, err
	}
	if !currentPortfolioAuthority(p.Authority) || !currentPortfolioAuthority(a.Authority) || p.Authority.Scope != a.Authority.Scope || a.BaseCurrency == "" || p.Portfolio == nil || p.Portfolio.BaseCurrency != a.BaseCurrency {
		return nil, errors.New("current consistent portfolio scope unavailable")
	}
	sectors := map[string]string{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for i, g := range p.ByUnderlying {
		if i >= 12 {
			break
		}
		if !portfolioClassificationUnambiguous(g.Underlying, p) {
			continue
		}
		contract, ok := rpc.UnderlyingMarketContract(g)
		if !ok || !rpc.ExpectsMarketDataGroup(g) {
			continue
		}
		wg.Go(func() {
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()
			route, _, _, e := normaliseStockQuoteContract(contract)
			if e != nil {
				return
			}
			name, e := c.MarketIndustry(ctx, route, 3*time.Second)
			if e != nil || name == "" {
				return
			}
			mu.Lock()
			sectors[g.Underlying] = name
			mu.Unlock()
		})
	}
	wg.Wait()
	if !c.SessionCurrent(binding) {
		return nil, errors.New("broker session changed during portfolio observation")
	}
	// Account selection is distinct from connector lifetime.
	if !sameBrokerScope(brokerStateScope{Account: a.Authority.Scope.AccountID, Mode: a.Authority.Scope.AccountMode}, s.currentBrokerStateScope()) {
		return nil, errors.New("portfolio scope changed")
	}
	return projectPortfolio(a, p, sectors), nil
}
func projectPortfolio(a *rpc.AccountResult, p *rpc.PositionsResult, sectors map[string]string) *rpc.PortfolioSnapshotResult {
	r := &rpc.PortfolioSnapshotResult{AsOf: time.Now(), AccountAsOf: a.AsOf, PositionsAsOf: p.AsOf, Authority: p.Authority, BaseCurrency: a.BaseCurrency, SectorBasis: "IBKR industry classification · signed held value / NLV; options use underlying classification", CoverageStatus: "complete"}
	valid := func(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
	if a.Authority.Fields != nil && a.Authority.Fields.NetLiquidation && valid(a.NetLiquidation) && a.NetLiquidation > 0 {
		v := a.NetLiquidation
		r.NetLiquidation = &v
	}
	classes := map[string]*rpc.PortfolioAllocation{}
	industry := map[string]*rpc.PortfolioAllocation{}
	add := func(rows map[string]*rpc.PortfolioAllocation, name string, v *float64) {
		x := rows[name]
		if x == nil {
			x = &rpc.PortfolioAllocation{Name: name}
			rows[name] = x
		}
		if v == nil || !valid(*v) {
			x.Missing++
			r.CoverageStatus = "partial"
			return
		}
		x.Observed++
		if x.ValueBase == nil {
			z := 0.0
			x.ValueBase = &z
		}
		*x.ValueBase += *v
	}
	var cash *float64
	if a.Authority.Fields != nil && a.Authority.Fields.TotalCash && valid(a.TotalCash) {
		v := a.TotalCash
		cash = &v
	}
	add(classes, "Cash", cash)
	cost := 0.0
	for _, row := range append(append([]rpc.PositionView{}, p.Stocks...), p.Options...) {
		class := "Other"
		switch strings.ToUpper(row.SecType) {
		case "STK", "STOCK":
			class = "Stocks"
		case "OPT", "OPTION":
			class = "Options"
		}
		value := row.MarketValueBase
		if row.Stale && rpc.ExpectsMarketData(row) {
			value = nil
		}
		add(classes, class, value)
		name := sectors[strings.ToUpper(row.Symbol)]
		if name == "" {
			name = "Unclassified"
			r.CoverageStatus = "partial"
		}
		add(industry, name, value)
		rate, ok := positionBaseRate(row, a.BaseCurrency)
		// IBKR average option cost already includes its contract multiplier.
		if class == "Other" || !ok || !valid(rate) || !valid(row.AvgCost) || row.AvgCost <= 0 || !valid(row.Quantity) {
			r.CostBasisMissing++
			continue
		}
		v := row.Quantity * row.AvgCost * rate
		if !valid(v) {
			r.CostBasisMissing++
			continue
		}
		cost += v
		r.CostBasisObserved++
	}
	finish := func(rows map[string]*rpc.PortfolioAllocation) []rpc.PortfolioAllocation {
		out := []rpc.PortfolioAllocation{}
		for _, x := range rows {
			if x.Missing == 0 && x.ValueBase != nil && r.NetLiquidation != nil {
				v := *x.ValueBase / *r.NetLiquidation * 100
				x.PercentNLV = &v
			}
			out = append(out, *x)
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
		return out
	}
	r.AssetClasses = finish(classes)
	r.Sectors = finish(industry)
	if r.NetLiquidation == nil || r.CostBasisMissing > 0 {
		r.CoverageStatus = "partial"
	}
	if r.CostBasisMissing == 0 && valid(cost) {
		r.CostBasisBase = &cost
	}
	return r
}

// Do not spread one exact listing's classification across ambiguous inventory.
func portfolioClassificationUnambiguous(symbol string, p *rpc.PositionsResult) bool {
	stockIDs := map[int]bool{}
	currencies := map[string]bool{}
	for _, row := range append(append([]rpc.PositionView{}, p.Stocks...), p.Options...) {
		if !strings.EqualFold(row.Symbol, symbol) {
			continue
		}
		if row.Currency == "" {
			return false
		}
		currencies[row.Currency] = true
		if row.SecType == "STOCK" || row.SecType == "STK" {
			if row.ConID <= 0 {
				return false
			}
			stockIDs[row.ConID] = true
		}
	}
	return len(currencies) == 1 && len(stockIDs) <= 1
}
