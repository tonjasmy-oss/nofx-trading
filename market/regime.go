package market

import (
	"math"
)

// MarketRegime represents the current market regime
type MarketRegime struct {
	Regime     string  // "trending" | "flat" | "volatile"
	Confidence float64 // 0.0 to 1.0
	ATR        float64 // ATR value
	ATRPct     float64 // ATR as percentage of price
	TrendSlope float64 // Trend slope (normalized)
	KeyLevels  KeyLevels
}

// KeyLevels represents support/resistance levels
type KeyLevels struct {
	Current      float64 // Current price
	Pivot        float64 // Pivot point
	Resistance1  float64 // R1
	Support1     float64 // S1
	Resistance2  float64 // R2
	Support2     float64 // S2
	RecentHigh   float64 // Recent high (20 period)
	RecentLow    float64 // Recent low (20 period)
}

// DetectRegime detects market regime using volatility and trend strength
// high, low, close: arrays of high, low, close prices
// lookback: number of periods for analysis (typically 50)
func DetectRegime(high, low, close []float64, lookback int) *MarketRegime {
	if len(close) < lookback || len(high) < lookback || len(low) < lookback {
		return &MarketRegime{
			Regime:     "flat",
			Confidence: 0.0,
			ATR:        0.0,
			ATRPct:     0.0,
			TrendSlope: 0.0,
			KeyLevels:  KeyLevels{},
		}
	}

	// Use only the lookback portion for calculations
	highArr := high[len(high)-lookback:]
	lowArr := low[len(low)-lookback:]
	closeArr := close[len(close)-lookback:]

	// Calculate ATR (14-period)
	period := 14
	if lookback < period {
		period = lookback
	}
	atr := calculateATRFromBars(highArr, lowArr, closeArr, period)
	atrVal := atr

	// Normalize ATR by price
	currentPrice := closeArr[len(closeArr)-1]
	atrPct := atrVal / currentPrice

	// Calculate trend slope using linear regression
	trendSlope := calculateTrendSlope(closeArr)

	// Calculate volatility threshold
	volThreshold := 0.02 // 2%
	trendThreshold := 0.1

	// Classify regime
	var regime string
	var confidence float64

	if atrPct > volThreshold*3 {
		regime = "volatile"
		confidence = math.Min(atrPct/(volThreshold*5), 1.0)
	} else if math.Abs(trendSlope) > trendThreshold {
		regime = "trending"
		confidence = math.Min(math.Abs(trendSlope)/(trendThreshold*3), 1.0)
	} else {
		regime = "flat"
		confidence = 0.5
	}

	// Calculate key levels
	levels := calculateKeyLevels(highArr, lowArr, closeArr, atrVal)

	return &MarketRegime{
		Regime:     regime,
		Confidence: confidence,
		ATR:        atrVal,
		ATRPct:     atrPct,
		TrendSlope: trendSlope,
		KeyLevels:  levels,
	}
}

// calculateATRFromBars calculates ATR from high/low/close arrays
func calculateATRFromBars(high, low, close []float64, period int) float64 {
	if len(high) < period+1 || len(low) < period+1 || len(close) < period+1 {
		return 0
	}

	trueRanges := make([]float64, 0, len(close)-1)
	for i := 1; i < len(close); i++ {
		hl := high[i] - low[i]
		hc := math.Abs(high[i] - close[i-1])
		lc := math.Abs(low[i] - close[i-1])
		tr := math.Max(hl, math.Max(hc, lc))
		trueRanges = append(trueRanges, tr)
	}

	if len(trueRanges) < period {
		return 0
	}

	// First ATR is simple average
	var sum float64
	for i := 0; i < period; i++ {
		sum += trueRanges[i]
	}
	atr := sum / float64(period)

	// Subsequent ATR values use smoothing
	for i := period; i < len(trueRanges); i++ {
		atr = (atr*float64(period-1) + trueRanges[i]) / float64(period)
	}

	return atr
}

// calculateTrendSlope calculates trend slope using linear regression
func calculateTrendSlope(prices []float64) float64 {
	if len(prices) < 2 {
		return 0
	}

	n := float64(len(prices))
	_ = (n - 1) // unused xMean, reserved for future use

	var sumX, sumY, sumXY, sumXX float64
	for i := 0; i < len(prices); i++ {
		x := float64(i)
		y := prices[i]
		sumX += x
		sumY += y
		sumXY += x * y
		sumXX += x * x
	}

	denom := n*sumXX - sumX*sumX
	if denom == 0 {
		return 0
	}

	// slope = (n*sumXY - sumX*sumY) / denom
	slope := (n*sumXY - sumX*sumY) / denom

	// Normalize by average price
	avgPrice := sumY / n
	if avgPrice == 0 {
		return 0
	}

	return slope / avgPrice
}

// calculateKeyLevels calculates support/resistance levels
func calculateKeyLevels(high, low, close []float64, atr float64) KeyLevels {
	if len(high) == 0 || len(low) == 0 || len(close) == 0 {
		return KeyLevels{}
	}

	currentPrice := close[len(close)-1]

	// Recent high/low (20 period lookback)
	recentHigh := high[0]
	recentLow := low[0]
	for i := 1; i < len(high) && i < 20; i++ {
		if high[i] > recentHigh {
			recentHigh = high[i]
		}
		if low[i] < recentLow {
			recentLow = low[i]
		}
	}

	// Pivot point (H+L+C)/3
	pivot := (high[len(high)-1] + low[len(low)-1] + close[len(close)-1]) / 3

	// Support and resistance levels based on ATR
	r1 := pivot + atr
	s1 := pivot - atr
	r2 := pivot + 2*atr
	s2 := pivot - 2*atr

	return KeyLevels{
		Current:     currentPrice,
		Pivot:       pivot,
		Resistance1: r1,
		Support1:    s1,
		Resistance2: r2,
		Support2:    s2,
		RecentHigh:  recentHigh,
		RecentLow:   recentLow,
	}
}