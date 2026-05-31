package kernel

import (
	"math"
)

// RSI calculates Relative Strength Index
// price: array of closing prices
// period: RSI period (typically 14)
func RSI(price []float64, period int) float64 {
	if len(price) < period+1 {
		return 50.0 // Not enough data, return neutral
	}

	var gains, losses float64
	for i := len(price) - period; i < len(price); i++ {
		delta := price[i] - price[i-1]
		if delta > 0 {
			gains += delta
		} else {
			losses += -delta
		}
	}

	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)

	if avgLoss == 0 {
		return 100.0 // No losses means RSI is 100
	}

	rs := avgGain / avgLoss
	rsi := 100.0 - (100.0 / (1.0 + rs))
	return rsi
}

// EMA calculates Exponential Moving Average
// price: array of closing prices
// period: EMA period
func EMA(price []float64, period int) float64 {
	if len(price) == 0 {
		return 0
	}
	if len(price) < period {
		period = len(price)
	}

	// Use SMA of first 'period' bars as initial EMA
	var sum float64
	for i := 0; i < period; i++ {
		sum += price[i]
	}
	ema := sum / float64(period)

	// Multiplier for EMA
	k := 2.0 / float64(period+1)

	// Calculate EMA
	for i := period; i < len(price); i++ {
		ema = (price[i]-ema)*k + ema
	}

	return ema
}

// MACDResult holds MACD calculation results
type MACDResult struct {
	MACD   float64 // MACD line (fast EMA - slow EMA)
	Signal float64 // Signal line (EMA of MACD)
	Hist   float64 // Histogram (MACD - Signal)
}

// MACD calculates Moving Average Convergence Divergence
// price: array of closing prices
// fast: fast EMA period (typically 12)
// slow: slow EMA period (typically 26)
// signal: signal EMA period (typically 9)
func MACD(price []float64, fast, slow, signalPeriod int) MACDResult {
	if len(price) < slow {
		return MACDResult{MACD: 0, Signal: 0, Hist: 0}
	}

	// Calculate fast and slow EMAs
	emaFast := EMA(price, fast)
	emaSlow := EMA(price, slow)
	macdLine := emaFast - emaSlow

	// Calculate signal line (EMA of MACD values)
	// Build MACD series
	macdSeries := make([]float64, 0, len(price)-slow+1)
	for i := slow; i <= len(price); i++ {
		ef := EMA(price[:i], fast)
		es := EMA(price[:i], slow)
		macdSeries = append(macdSeries, ef-es)
	}

	if len(macdSeries) < signalPeriod {
		return MACDResult{MACD: macdLine, Signal: macdLine, Hist: 0}
	}

	signalLine := EMA(macdSeries, signalPeriod)
	hist := macdLine - signalLine

	return MACDResult{
		MACD:   macdLine,
		Signal: signalLine,
		Hist:   hist,
	}
}

// BollingerBandsResult holds Bollinger Bands calculation results
type BollingerBandsResult struct {
	Upper  float64 // Upper band
	Middle float64 // Middle band (SMA)
	Lower  float64 // Lower band
}

// BollingerBands calculates Bollinger Bands
// price: array of closing prices
// period: rolling period (typically 20)
// stdMult: standard deviation multiplier (typically 2.0)
func BollingerBands(price []float64, period int, stdMult float64) BollingerBandsResult {
	if len(price) < period {
		return BollingerBandsResult{Upper: 0, Middle: 0, Lower: 0}
	}

	// Calculate SMA
	var sum float64
	for i := len(price) - period; i < len(price); i++ {
		sum += price[i]
	}
	sma := sum / float64(period)

	// Calculate standard deviation
	var sumSq float64
	for i := len(price) - period; i < len(price); i++ {
		diff := price[i] - sma
		sumSq += diff * diff
	}
	stdDev := math.Sqrt(sumSq / float64(period))

	upper := sma + stdMult*stdDev
	lower := sma - stdMult*stdDev

	return BollingerBandsResult{
		Upper:  upper,
		Middle: sma,
		Lower:  lower,
	}
}

// ATR calculates Average True Range
// high, low, close: arrays of high, low, close prices
// period: ATR period (typically 14)
func ATR(high, low, close []float64, period int) float64 {
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

	// Calculate ATR using smoothed average (similar to TradingView)
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

// VolumeRatio calculates volume ratio relative to average
// volumes: array of volume values
// period: lookback period for average calculation
func VolumeRatio(volumes []float64, period int) float64 {
	if len(volumes) < period {
		return 1.0 // Not enough data, return neutral
	}

	// Calculate average volume over lookback period
	var sum float64
	startIdx := len(volumes) - period
	for i := startIdx; i < len(volumes); i++ {
		sum += volumes[i]
	}
	avgVolume := sum / float64(period)

	if avgVolume == 0 {
		return 1.0
	}

	currentVolume := volumes[len(volumes)-1]
	return currentVolume / avgVolume
}

// SMA calculates Simple Moving Average
func SMA(price []float64, period int) float64 {
	if len(price) < period {
		return 0
	}
	var sum float64
	for i := len(price) - period; i < len(price); i++ {
		sum += price[i]
	}
	return sum / float64(period)
}

// StdDev calculates standard deviation
func StdDev(price []float64, period int) float64 {
	if len(price) < period {
		return 0
	}
	mean := SMA(price, period)
	var sumSq float64
	for i := len(price) - period; i < len(price); i++ {
		diff := price[i] - mean
		sumSq += diff * diff
	}
	return math.Sqrt(sumSq / float64(period))
}

// SuperTrendResult holds SuperTrend calculation results
type SuperTrendResult struct {
	UpperBand float64 // Upper band (basis + multiplier * ATR)
	LowerBand float64 // Lower band (basis - multiplier * ATR)
	Trend     int     // 1 for uptrend, -1 for downtrend
	Value     float64 // Current SuperTrend value
}

// SuperTrend calculates SuperTrend indicator
// high, low, close: arrays of high, low, close prices
// period: ATR period (typically 10)
// multiplier: multiplier for ATR (typically 3.0)
func SuperTrend(high, low, close []float64, period int, multiplier float64) SuperTrendResult {
	if len(high) < period+1 || len(low) < period+1 || len(close) < period+1 {
		return SuperTrendResult{}
	}
	atr := ATR(high, low, close, period)
	if atr == 0 {
		return SuperTrendResult{}
	}

	// Calculate basis (typical price = (high + low + close) / 3)
	n := len(close)
	basis := (high[n-1] + low[n-1] + close[n-1]) / 3.0

	upperBand := basis + multiplier*atr
	lowerBand := basis - multiplier*atr

	// Simple final smoothing: trend follows close vs bands
	trend := 1
	if close[n-1] < lowerBand {
		trend = -1
	}

	return SuperTrendResult{
		UpperBand: upperBand,
		LowerBand: lowerBand,
		Trend:     trend,
		Value:     basis,
	}
}

// KDJResult holds KDJ (Stochastic RSI variant) calculation results
type KDJResult struct {
	K float64 // Fast stochastic %K line
	D float64 // Slow signal line (%D)
	J float64 // J line (3*K - 2*D)
}

// KDJ calculates KDJ indicator
// high, low, close: arrays of high, low, close prices
// period: lookback period (typically 9)
// kPeriod: %K smoothing period (typically 3)
// dPeriod: %D smoothing period (typically 3)
func KDJ(high, low, close []float64, period, kPeriod, dPeriod int) KDJResult {
	if len(high) < period+1 || len(low) < period+1 || len(close) < period+1 {
		return KDJResult{}
	}

	n := len(close)
	// Find highest high and lowest low over period
	var highestHigh, lowestLow float64
	for i := n - period; i < n; i++ {
		if i == n-period || high[i] > highestHigh {
			highestHigh = high[i]
		}
		if i == n-period || low[i] < lowestLow {
			lowestLow = low[i]
		}
	}

	rsv := 0.0
	if highestHigh != lowestLow {
		rsv = (close[n-1] - lowestLow) / (highestHigh - lowestLow) * 100.0
	}

	// K = SMA of RSV (simplified: use EMA-like smoothing)
	k := rsv
	for i := 0; i < kPeriod && n-kPeriod-1-i >= 0; i++ {
		// Simple smoothing via EMA-style
		k = k*2.0/3.0 + rsv*(1.0-2.0/3.0)
	}

	// D = SMA of K
	d := k
	for i := 0; i < dPeriod && n-dPeriod-1-i >= 0; i++ {
		d = d*2.0/3.0 + k*(1.0-2.0/3.0)
	}

	// J = 3*K - 2*D
	j := 3*k - 2*d

	return KDJResult{K: k, D: d, J: j}
}

// VWAP calculates Volume Weighted Average Price
// high, low, close, volume: arrays of OHLCV prices
func VWAP(high, low, close, volume []float64) float64 {
	if len(high) != len(low) || len(high) != len(close) || len(high) != len(volume) {
		return 0
	}
	if len(high) == 0 {
		return 0
	}

	var cumVP, cumV float64
	for i := 0; i < len(close); i++ {
		typicalPrice := (high[i] + low[i] + close[i]) / 3.0
		cumVP += typicalPrice * volume[i]
		cumV += volume[i]
	}
	if cumV == 0 {
		return 0
	}
	return cumVP / cumV
}

// BollingerBandsPercentB calculates Bollinger Bands %B
// price: current close price
// bands: BollingerBandsResult from BollingerBands()
// Returns: (close - lower) / (upper - lower), normalized 0-1 range
func BollingerBandsPercentB(price float64, bands BollingerBandsResult) float64 {
	bandWidth := bands.Upper - bands.Lower
	if bandWidth == 0 {
		return 0.5
	}
	return (price - bands.Lower) / bandWidth
}

// ADXResult holds ADX (Average Directional Index) calculation results
type ADXResult struct {
	ADX   float64 // Average Directional Index (trend strength)
	PlusDI float64 // Plus Directional Indicator (+DI)
	MinusDI float64 // Minus Directional Indicator (-DI)
}

// ADX calculates Average Directional Index
// high, low, close: arrays of high, low, close prices
// period: ADX period (typically 14)
func ADX(high, low, close []float64, period int) ADXResult {
	if len(high) < period+2 || len(low) < period+2 || len(close) < period+2 {
		return ADXResult{}
	}

	n := len(close)
	// Calculate True Range components
	var plusDM, minusDM, tr float64
	for i := 1; i <= period; i++ {
		hl := high[i] - low[i]
		hc := math.Abs(high[i] - close[i-1])
		lc := math.Abs(low[i] - close[i-1])
		tr += math.Max(hl, math.Max(hc, lc))

		highDiff := high[i] - high[i-1]
		lowDiff := low[i-1] - low[i]

		if highDiff > lowDiff && highDiff > 0 {
			plusDM += highDiff
		}
		if lowDiff > highDiff && lowDiff > 0 {
			minusDM += lowDiff
		}
	}

	if tr == 0 {
		return ADXResult{}
	}

	// Smoothed +DI and -DI
	plusDI := (plusDM / tr) * 100.0
	minusDI := (minusDM / tr) * 100.0

	// DX = |+DI - -DI| / (|+DI + -DI|) * 100
	diDiff := math.Abs(plusDI - minusDI)
	diSum := plusDI + minusDI
	if diSum == 0 {
		return ADXResult{ADX: 0, PlusDI: plusDI, MinusDI: minusDI}
	}
	dx := (diDiff / diSum) * 100.0

	// ADX is smoothed DX over period
	adx := dx
	for i := period + 1; i < n && i-period-1 >= 0; i++ {
		// Simple EMA smoothing for ADX
		adx = adx*(1.0-2.0/float64(period+1)) + dx*(2.0/float64(period+1))
	}

	return ADXResult{ADX: adx, PlusDI: plusDI, MinusDI: minusDI}
}