package risk

import (
	"math"
	"testing"
)

func completeRoleFixture() RuleInputs {
	return RuleInputs{BaseCurrency: "EUR", Positions: SourceState{Healthy: true}, Names: []NameInput{
		{Symbol: "LONG", ExposureBaseComplete: true, ExposureBase: 10000},
		{Symbol: "SPY", ExposureBaseComplete: true, ExposureBase: -1000, Legs: []LegInput{{Right: "P", Quantity: 1, Multiplier: 100, Underlying: new(100.0), Delta: new(-0.5), FXToBase: new(0.2), HedgeListed: true}}},
	}}
}

func TestCompleteIndexPutRolesCurrencyBoundaryAndImmutability(t *testing.T) {
	pol := DefaultRulebookPolicy()
	// Native notional = 5000, but base notional = 1000. Put the denominator
	// at a boundary where ignoring FX would change the disposition.
	band := 2 * math.Max(pol.RegimeCalm.HedgeBandMaxPct, math.Max(pol.RegimeEarlyWarning.HedgeBandMaxPct, pol.RegimeConfirmed.HedgeBandMaxPct))
	in := completeRoleFixture()
	in.Names[0].ExposureBase = 1000 / (band / 100)
	spot, fx := in.Names[1].Legs[0].Underlying, in.Names[1].Legs[0].FXToBase
	got, ok := ClassifyCompleteIndexPutRoles(in, pol)
	if !ok || got.Names[1].Legs[0].IndexPutRole != IndexPutRoleProtection {
		t.Fatal("FX unit mismatch removed protection")
	}
	in.Names[0].ExposureBase *= 0.99
	got, ok = ClassifyCompleteIndexPutRoles(in, pol)
	if !ok || got.Names[1].Legs[0].IndexPutRole != IndexPutRoleDirectional {
		t.Fatal("complete directional evidence did not clear")
	}
	if in.Names[1].Legs[0].IndexPutRole != "" || *spot != 100 || *fx != 0.2 || got.Names[1].Legs[0].Underlying != spot || got.Names[1].Legs[0].FXToBase != fx {
		t.Fatal("classifier mutated native spot/FX input")
	}
}

func TestCompleteIndexPutRolesRejectsPartialAndNonFiniteBook(t *testing.T) {
	for name, mutate := range map[string]func(*RuleInputs){
		"incomplete_long":       func(in *RuleInputs) { in.Names[0].ExposureBaseComplete = false },
		"unhealthy_scope":       func(in *RuleInputs) { in.Positions.Healthy = false },
		"missing_delta":         func(in *RuleInputs) { in.Names[1].Legs[0].Delta = nil },
		"missing_spot":          func(in *RuleInputs) { in.Names[1].Legs[0].Underlying = nil },
		"missing_fx":            func(in *RuleInputs) { in.Names[1].Legs[0].FXToBase = nil },
		"stock_spot_substitute": func(in *RuleInputs) { in.Names[1].Legs[0].UnderlyingSource = UnderlyingSourceStockLegMark },
		"nan":                   func(in *RuleInputs) { in.Names[0].ExposureBase = math.NaN() },
		"wrong_sign":            func(in *RuleInputs) { in.Names[1].Legs[0].Delta = new(0.5) },
	} {
		t.Run(name, func(t *testing.T) {
			in := completeRoleFixture()
			mutate(&in)
			if _, ok := ClassifyCompleteIndexPutRoles(in, DefaultRulebookPolicy()); ok {
				t.Fatal("partial/invalid book classified")
			}
		})
	}
}
