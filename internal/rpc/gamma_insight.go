package rpc

import (
	"math"
	"time"
)

// GammaProfileMetrics describes the sampled model at its actual observation
// spot. The containing pointer is nil when these measurements are unavailable.
// Signed GEX assumes calls positive and puts negative; it is not dealer inventory.
type GammaProfileMetrics struct {
	GEXAtSpot      float64   `json:"gex_at_spot"`
	GrossGEXAtSpot float64   `json:"gross_gex_at_spot"`
	NetToGrossPct  float64   `json:"net_to_gross_pct"`
	Crossings      []float64 `json:"crossings"`
	ResolutionPct  float64   `json:"resolution_pct"`
}

// OptionSkew measures same-expiry 25-delta put minus call implied volatility.
// Nil IV/skew fields mean insufficient eligible, bracketing IV observations.
// Positive skew prices downside protection more richly; it is not a bearish
// probability or evidence of customer/dealer buy/sell direction.
type OptionSkew struct {
	TradingClass     string    `json:"trading_class"`
	Expiry           string    `json:"expiry"`
	DTE              float64   `json:"dte"`
	PutIV            *float64  `json:"put_iv,omitempty"`
	CallIV           *float64  `json:"call_iv,omitempty"`
	SkewVolPoints    *float64  `json:"skew_vol_points,omitempty"`
	PutDeltaBracket  []float64 `json:"put_delta_bracket,omitempty"`
	CallDeltaBracket []float64 `json:"call_delta_bracket,omitempty"`
	Status           string    `json:"status"`
	Reason           string    `json:"reason,omitempty"`
}

// GammaHorizonInsight preserves each expiry horizon's own local model reading.
type GammaHorizonInsight struct {
	Horizon       string   `json:"horizon"`
	Regime        string   `json:"regime"`
	GEXAtSpot     *float64 `json:"gex_at_spot,omitempty"`
	GrossSharePct *float64 `json:"gross_share_pct,omitempty"`
}

// GammaInsight is a bounded, daemon-authored explanation shared by Brief,
// CLI, MCP and the app. Quality governs use of these dated observations.
type GammaInsight struct {
	Underlying            string                `json:"underlying"`
	SpotAt                time.Time             `json:"spot_at,omitzero"`
	AsOf                  time.Time             `json:"as_of"`
	DataType              string                `json:"data_type"`
	Method                string                `json:"method"`
	Rankability           string                `json:"rankability"`
	RankabilityReason     string                `json:"rankability_reason,omitempty"`
	Regime                string                `json:"regime"`
	Interpretation        string                `json:"interpretation"`
	Provenance            string                `json:"provenance"`
	PositioningAssumption string                `json:"positioning_assumption"`
	DirectionalInference  string                `json:"directional_inference"`
	SkewInterpretation    string                `json:"skew_interpretation"`
	SelectedSkew          *OptionSkew           `json:"selected_skew,omitempty"`
	Horizons              []GammaHorizonInsight `json:"horizons"`
	HorizonInterpretation string                `json:"horizon_interpretation"`
	HorizonAgreement      string                `json:"horizon_agreement,omitempty"`
}

// GammaComputedRegime classifies the local model, preserving the established
// transition-distance threshold. Geometric above/below is not gamma sign.
// The legacy fallback is for older typed callers; current computes emit metrics.
func GammaComputedRegime(c *GammaZeroComputed) string {
	if c == nil {
		return ""
	}
	if c.ProfileMetrics == nil {
		if c.GapPct != nil {
			return GammaRegimeFromGap(c.GapPct)
		}
		return GammaBucketRegime(c.SpotUnderlying, c.ZeroGamma, c.GammaSign)
	}
	return GammaMeasuredRegime(c.SpotUnderlying, c.ZeroGamma, c.GammaSign, c.ProfileMetrics)
}

// GammaMeasuredRegime applies the same local-sign rule to one horizon.
func GammaMeasuredRegime(spot float64, zero *float64, sign string, metrics *GammaProfileMetrics) string {
	if metrics == nil {
		return GammaBucketRegime(spot, zero, sign)
	}
	if metrics.GEXAtSpot == 0 && metrics.GrossGEXAtSpot <= 0 {
		return ""
	}
	if math.IsNaN(metrics.GEXAtSpot) || math.IsInf(metrics.GEXAtSpot, 0) {
		return ""
	}
	if zero != nil && *zero > 0 && math.Abs((spot-*zero) / *zero * 100) <= GammaTransitionGapPct {
		return "transition_gamma"
	}
	if metrics.GEXAtSpot > 0 {
		return "long_gamma"
	}
	if metrics.GEXAtSpot < 0 {
		return "short_gamma"
	}
	return "transition_gamma"
}

// CloneGammaInsight gives adapters an independent bounded value, including
// optional skew/bracket and horizon measurements.
func CloneGammaInsight(in *GammaInsight) *GammaInsight {
	if in == nil {
		return nil
	}
	out := *in
	out.Horizons = append([]GammaHorizonInsight(nil), in.Horizons...)
	for i := range out.Horizons {
		if v := out.Horizons[i].GEXAtSpot; v != nil {
			out.Horizons[i].GEXAtSpot = new(*v)
		}
		if v := out.Horizons[i].GrossSharePct; v != nil {
			out.Horizons[i].GrossSharePct = new(*v)
		}
	}
	if in.SelectedSkew != nil {
		s := *in.SelectedSkew
		if s.PutIV != nil {
			s.PutIV = new(*s.PutIV)
		}
		if s.CallIV != nil {
			s.CallIV = new(*s.CallIV)
		}
		if s.SkewVolPoints != nil {
			s.SkewVolPoints = new(*s.SkewVolPoints)
		}
		s.PutDeltaBracket = append([]float64(nil), s.PutDeltaBracket...)
		s.CallDeltaBracket = append([]float64(nil), s.CallDeltaBracket...)
		out.SelectedSkew = &s
	}
	return &out
}

// CloneGammaProfileMetrics copies the detected crossings as well as scalars.
func CloneGammaProfileMetrics(in *GammaProfileMetrics) *GammaProfileMetrics {
	if in == nil {
		return nil
	}
	out := *in
	out.Crossings = append([]float64(nil), in.Crossings...)
	return &out
}

// CloneGammaSignalQuality keeps adapter copies independent of cache refreshes.
func CloneGammaSignalQuality(in *GammaSignalQuality) *GammaSignalQuality {
	if in == nil {
		return nil
	}
	out := *in
	out.Gates = append([]GammaQualityGate(nil), in.Gates...)
	out.Blockers = append([]string(nil), in.Blockers...)
	out.Context = append([]string(nil), in.Context...)
	if in.ByUnderlying != nil {
		out.ByUnderlying = make(map[string]GammaSignalQuality, len(in.ByUnderlying))
		for k, v := range in.ByUnderlying {
			out.ByUnderlying[k] = *CloneGammaSignalQuality(&v)
		}
	}
	return &out
}

func compactGammaInsights(c *GammaZeroComputed) []*GammaInsight {
	if c == nil {
		return nil
	}
	if c.Scope != GammaZeroScopeCombined {
		if c.Insight == nil {
			return nil
		}
		return []*GammaInsight{CloneGammaInsight(c.Insight)}
	}
	out := []*GammaInsight{}
	for _, key := range []string{"SPX", "SPY"} {
		if sub := c.PerIndex[key]; sub != nil && sub.Insight != nil {
			out = append(out, CloneGammaInsight(sub.Insight))
		}
	}
	return out
}
