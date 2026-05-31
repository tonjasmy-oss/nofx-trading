package backtest

import (
	"context"
	"time"

	"nofx/provider/coinank/coinank_api"
	"nofx/provider/coinank/coinank_enum"
)

// Run is a convenience wrapper that fetches klines, builds a strategy, and runs the backtest.
// It provides a simple single-call interface for the AI agent tool.
func Run(cfg BacktestConfig) (*Result, error) {
	ctx := context.Background()

	// Map exchange string to coinank enum
	exchange := coinankEnumExchange(cfg.Exchange)
	interval := coinank_enum.Interval(string(cfg.Timeframe))

	// Default time range: last 30 days
	endTime := cfg.EndTime
	if endTime.IsZero() {
		endTime = time.Now()
	}
	startTime := cfg.StartTime
	if startTime.IsZero() {
		startTime = endTime.AddDate(0, 0, -30)
	}

	// Fetch klines from CoinAnk
	// coinank_api.Kline(ctx, symbol, exchange, endTime, side, limit, interval)
	klines, err := coinank_api.Kline(ctx, cfg.Symbol, exchange, endTime.UnixMilli(), coinank_enum.To, 1000, interval)
	if err != nil {
		return nil, err
	}
	if len(klines) == 0 {
		return nil, &BacktestError{Message: "no klines returned from exchange"}
	}

	// Convert to KlineBar (newest first)
	bars := make([]KlineBar, len(klines))
	for i, k := range klines {
		rev := len(klines) - 1 - i
		bars[rev] = KlineBar{
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
	filtered := make([]KlineBar, 0, len(bars))
	for _, b := range bars {
		if b.Time >= startMs && b.Time <= endMs {
			filtered = append(filtered, b)
		}
	}
	if len(filtered) == 0 {
		return nil, &BacktestError{Message: "no klines in the specified time range"}
	}

	// Build default strategy if not set
	if cfg.Strategy == nil {
		cfg.Strategy = NewMACrossStrategy(10, 30)
	}

	return RunBacktest(cfg, filtered)
}

// coinankEnumExchange maps exchange name to coinank enum
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
	default:
		return coinank_enum.Binance
	}
}

// BacktestError represents a backtest-specific error
type BacktestError struct {
	Message string
}

func (e *BacktestError) Error() string {
	return e.Message
}

// maCrossStrategy is a simple moving average cross strategy
type maCrossStrategy struct {
	fast, slow int
}

// NewMACrossStrategy creates a new moving average cross strategy.
func NewMACrossStrategy(fast, slow int) Strategy {
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

func (s *maCrossStrategy) OnBar(bar KlineBar) []OrderSignal {
	// Stub: real implementation tracks price history
	// For now returns no signal (requires price history buffer)
	return nil
}