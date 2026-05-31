package api

import (
	"net/http"
	"time"

	"nofx/pkg/backtest"
	"nofx/provider/coinank/coinank_api"
	"nofx/provider/coinank/coinank_enum"

	"github.com/gin-gonic/gin"
)

// handleBacktest runs a backtest and returns results.
// POST /api/backtest
// Body: { "symbol": "BTC/USDT", "exchange": "binance", "interval": "1h",
//         "start_time": 1704067200000, "end_time": 1706745600000,
//         "initial_capital": 10000, "fee_rate": 0.001,
//         "strategy": { "type": "ma_cross", "fast_period": 10, "slow_period": 30 } }
func (s *Server) handleBacktest(c *gin.Context) {
	var req backtestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
		return
	}

	// Validate required fields
	if req.Symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbol is required"})
		return
	}
	if req.Exchange == "" {
		req.Exchange = "binance"
	}
	if req.Interval == "" {
		req.Interval = "1h"
	}
	if req.InitialCapital <= 0 {
		req.InitialCapital = 10000
	}
	if req.FeeRate <= 0 {
		req.FeeRate = 0.001
	}

	// Fetch klines
	startTime := time.UnixMilli(req.StartTime)
	endTime := time.UnixMilli(req.EndTime)
	if endTime.IsZero() {
		endTime = time.Now()
	}

	ce := coinankEnumExchange(req.Exchange)
	ci := coinank_enum.Interval(req.Interval)

	ctx := c.Request.Context()
	klines, err := coinank_api.Kline(ctx, req.Symbol, ce, endTime.UnixMilli(), coinank_enum.To, 1000, ci)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch klines: " + err.Error()})
		return
	}

	if len(klines) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no klines returned for the given time range"})
		return
	}

	// Convert to backtest KlineBar
	bars := make([]backtest.KlineBar, len(klines))
	for i, k := range klines {
		// newest first → reverse
		rev := len(klines) - 1 - i
		bars[rev] = backtest.KlineBar{
			Time:   k.StartTime,
			Open:   k.Open,
			High:   k.High,
			Low:    k.Low,
			Close:  k.Close,
			Volume: k.Volume,
		}
	}

	// Filter by time range
	startMs := startTime.UnixMilli()
	endMs := endTime.UnixMilli()
	filtered := make([]backtest.KlineBar, 0, len(bars))
	for _, b := range bars {
		if b.Time >= startMs && b.Time <= endMs {
			filtered = append(filtered, b)
		}
	}

	if len(filtered) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no klines in the specified time range"})
		return
	}

	// Build strategy from request
	strategy := buildStrategy(req.Strategy)

	cfg := backtest.BacktestConfig{
		Symbol:          req.Symbol,
		Exchange:       req.Exchange,
		Timeframe:      backtest.Timeframe(req.Interval),
		StartTime:      startTime,
		EndTime:        endTime,
		InitialCapital: req.InitialCapital,
		Leverage:       req.Leverage,
		FeeRate:        req.FeeRate,
		Slippage:       req.Slippage,
		MinTradeSize:   req.MinTradeSize,
		StopLossPct:    req.StopLossPct,
		TakeProfitPct:  req.TakeProfitPct,
		Strategy:      strategy,
	}

	result, err := backtest.RunBacktest(cfg, filtered)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "backtest failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

type backtestRequest struct {
	Symbol         string  `json:"symbol"`
	Exchange      string  `json:"exchange"`
	Interval      string  `json:"interval"`
	StartTime     int64   `json:"start_time"`
	EndTime       int64   `json:"end_time"`
	InitialCapital float64 `json:"initial_capital"`
	Leverage      int     `json:"leverage"`
	FeeRate       float64 `json:"fee_rate"`
	Slippage      float64 `json:"slippage"`
	MinTradeSize  float64 `json:"min_trade_size"`
	StopLossPct   float64 `json:"stop_loss_pct"`
	TakeProfitPct float64 `json:"take_profit_pct"`
	Strategy      strategyRequest `json:"strategy"`
}

type strategyRequest struct {
	Type        string  `json:"type"`
	FastPeriod  int     `json:"fast_period"`
	SlowPeriod  int     `json:"slow_period"`
	Period      int     `json:"period"`
	Multipliers  []float64 `json:"multipliers"`
}

func buildStrategy(req strategyRequest) backtest.Strategy {
	switch req.Type {
	case "ma_cross":
		return newMACrossStrategy(req.FastPeriod, req.SlowPeriod)
	default:
		return newMACrossStrategy(10, 30)
	}
}

// MACrossStrategy is a simple moving average cross strategy.
type maCrossStrategy struct {
	fast, slow int
	prevFast  float64
	prevSlow  float64
}

func newMACrossStrategy(fast, slow int) *maCrossStrategy {
	if fast <= 0 {
		fast = 10
	}
	if slow <= 0 {
		slow = 30
	}
	if fast >= slow {
		fast = slow / 2
		if fast == 0 {
			fast = 10
		}
	}
	return &maCrossStrategy{fast: fast, slow: slow}
}

func (s *maCrossStrategy) OnBar(bar backtest.KlineBar) []backtest.OrderSignal {
	// Simple MA cross: hold long when fast MA > slow MA
	// (simplified — real impl would track price history)
	return nil // stub: requires price history buffer
}

func coinankEnumExchange(exchange string) coinank_enum.Exchange {
	switch exchange {
	case "binance":
		return coinank_enum.Binance
	case "bybit":
		return coinank_enum.Bybit
	case "okx":
		return coinank_enum.Okex
	case "bitget":
		return coinank_enum.Bitget
	case "gate":
		return coinank_enum.Gate
	case "hyperliquid":
		return coinank_enum.Hyperliquid
	case "aster":
		return coinank_enum.Aster
	case "huobi":
		return coinank_enum.Huobi
	default:
		return coinank_enum.Binance
	}
}