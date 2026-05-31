package strategy

import (
	"encoding/json"
	"fmt"

	"nofx/kernel"
	"nofx/pkg/backtest"
)

// Script represents a declarative strategy defined in JSON/YAML
type Script struct {
	Name       string        `json:"name"`
	Type       string        `json:"type"`
	Parameters ScriptParams `json:"parameters"`
	Rules      []Rule       `json:"rules"`
}

// ScriptParams holds indicator parameters for a scripted strategy
type ScriptParams struct {
	FastPeriod  int           `json:"fast_period"`
	SlowPeriod int           `json:"slow_period"`
	Period     int           `json:"period"`
	Multiplier float64       `json:"multiplier"`
	StdMult    float64       `json:"std_mult"`
	KPeriod    int           `json:"k_period"`
	DPeriod    int           `json:"d_period"`
	BuyVolumeMin float64    `json:"buy_volume_min"`
}

// Rule represents a single entry/exit rule
type Rule struct {
	Type       string   `json:"type"`   // "crossover", "crossunder", "above", "below", "value"
	IndicatorA string   `json:"indicator_a"`
	IndicatorB string   `json:"indicator_b"`
	Target    string   `json:"target"` // "indicator_a", "price", "indicator_b"
	Threshold float64  `json:"threshold"`
	Direction string   `json:"direction"` // "long", "short"
}

// ParseScript parses a JSON strategy script into a backtest.Strategy
func ParseScript(data []byte) (backtest.Strategy, error) {
	var scr Script
	if err := json.Unmarshal(data, &scr); err != nil {
		return nil, fmt.Errorf("parse script: %w", err)
	}
	return NewScriptStrategy(scr), nil
}

// NewScriptStrategy creates a backtest.Strategy from a Script
func NewScriptStrategy(scr Script) backtest.Strategy {
	return &scriptStrategy{script: scr}
}

type scriptStrategy struct {
	script Script
}

func (s *scriptStrategy) OnBar(bar backtest.KlineBar) []backtest.OrderSignal {
	// A real implementation would track price history across bars.
	// Here we return no signals (stub) — full bar history is needed.
	return nil
}

// NewMACrossStrategy is exported from backtest for use by script parser
func NewMACrossStrategy(fast, slow int) backtest.Strategy {
	return backtest.NewMACrossStrategy(fast, slow)
}

// IndicatorFunc defines a function that computes an indicator from OHLCV
type IndicatorFunc func(high, low, close, volume []float64) float64

// BuiltInIndicators maps indicator names to functions
var BuiltInIndicators = map[string]IndicatorFunc{
	"RSI": func(high, low, close, volume []float64) float64 {
		return kernel.RSI(close, 14)
	},
	"EMA_10": func(high, low, close, volume []float64) float64 {
		return kernel.EMA(close, 10)
	},
	"EMA_30": func(high, low, close, volume []float64) float64 {
		return kernel.EMA(close, 30)
	},
	"SMA_20": func(high, low, close, volume []float64) float64 {
		return kernel.SMA(close, 20)
	},
	"SMA_50": func(high, low, close, volume []float64) float64 {
		return kernel.SMA(close, 50)
	},
	"BB_upper": func(high, low, close, volume []float64) float64 {
		return kernel.BollingerBands(close, 20, 2.0).Upper
	},
	"BB_lower": func(high, low, close, volume []float64) float64 {
		return kernel.BollingerBands(close, 20, 2.0).Lower
	},
	"ATR": func(high, low, close, volume []float64) float64 {
		return kernel.ATR(high, low, close, 14)
	},
	"VolumeRatio": func(high, low, close, volume []float64) float64 {
		return kernel.VolumeRatio(volume, 20)
	},
}

// ExampleScript returns a sample strategy script in JSON format
func ExampleScript() string {
	return `{
  "name": "MA Cross + RSI Filter",
  "type": "ma_cross",
  "parameters": {
    "fast_period": 10,
    "slow_period": 30
  },
  "rules": [
    {
      "type": "crossover",
      "indicator_a": "EMA_10",
      "indicator_b": "EMA_30",
      "direction": "long"
    },
    {
      "type": "crossunder",
      "indicator_a": "EMA_10",
      "indicator_b": "EMA_30",
      "direction": "short"
    }
  ]
}`
}