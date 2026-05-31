package experiment

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"nofx/kernel"
	"nofx/pkg/backtest"
)

// Config holds experiment configuration
type Config struct {
	Name           string        `json:"name"`
	Symbol         string        `json:"symbol"`
	Exchange       string        `json:"exchange"`
	Interval       string        `json:"interval"`
	StartTime      time.Time     `json:"start_time"`
	EndTime        time.Time     `json:"end_time"`
	InitialCapital float64       `json:"initial_capital"`
	ParameterSpace []Parameter   `json:"parameter_space"`
	MetricTarget   string        `json:"metric_target"` // "sharpe_ratio", "total_return", "profit_factor"
	Mode           string        `json:"mode"`          // "grid", "random"
	MaxTrials      int           `json:"max_trials"`
}

// Parameter defines a parameter to search over
type Parameter struct {
	Name   string   `json:"name"`
	Type   string   `json:"type"`   // "int", "float"
	Min    float64  `json:"min"`
	Max    float64  `json:"max"`
	Step   float64  `json:"step"`
	Values []string `json:"values"` // for categorical
}

// Trial represents a single backtest trial with a parameter set
type Trial struct {
	ID         int             `json:"id"`
	Params     map[string]any  `json:"params"`
	Metrics    backtest.Metrics `json:"metrics"`
	Status     string          `json:"status"` // "running", "completed", "failed"
	Error      string          `json:"error,omitempty"`
	DurationMs int64           `json:"duration_ms"`
}

// Result holds the experiment result with all trials ranked
type Result struct {
	Config       Config   `json:"config"`
	Trials       []Trial  `json:"trials"`
	BestTrial    *Trial   `json:"best_trial"`
	BestParams   map[string]any `json:"best_params"`
	BestMetric   float64  `json:"best_metric"`
	TotalTrials  int      `json:"total_trials"`
	CompletedTrials int   `json:"completed_trials"`
	DurationMs   int64    `json:"duration_ms"`
}

// Run executes an experiment (grid or random search)
func Run(ctx context.Context, cfg Config) (*Result, error) {
	if cfg.MaxTrials <= 0 {
		cfg.MaxTrials = 20
	}
	if cfg.InitialCapital <= 0 {
		cfg.InitialCapital = 10000
	}
	if cfg.Exchange == "" {
		cfg.Exchange = "binance"
	}
	if cfg.Interval == "" {
		cfg.Interval = "1h"
	}

	result := &Result{
		Config: cfg,
		Trials: make([]Trial, 0, cfg.MaxTrials),
	}

	start := time.Now()

	switch cfg.Mode {
	case "random":
		result.Trials, result.BestTrial, result.BestParams, result.BestMetric = runRandomSearch(ctx, cfg)
	case "grid":
		result.Trials, result.BestTrial, result.BestParams, result.BestMetric = runGridSearch(ctx, cfg)
	default:
		return nil, fmt.Errorf("unknown mode: %s (use 'grid' or 'random')", cfg.Mode)
	}

	result.TotalTrials = len(result.Trials)
	for _, t := range result.Trials {
		if t.Status == "completed" {
			result.CompletedTrials++
		}
	}
	result.DurationMs = time.Since(start).Milliseconds()

	return result, nil
}

func runGridSearch(ctx context.Context, cfg Config) ([]Trial, *Trial, map[string]any, float64) {
	trials := make([]Trial, 0, cfg.MaxTrials)
	bestTrial := (*Trial)(nil)
	bestParams := map[string]any{}
	bestMetric := -1e9

	id := 0
nextGrid:
	for _, combo := range gridCombinations(cfg.ParameterSpace) {
		if id >= cfg.MaxTrials {
			break
		}
		params := map[string]any{}
		for i, p := range cfg.ParameterSpace {
			params[p.Name] = combo[i]
		}

		trial := runSingleTrial(ctx, cfg, params, id)
		trials = append(trials, trial)
		id++

		if trial.Status == "completed" {
			metricVal := metricValue(trial.Metrics, cfg.MetricTarget)
			if metricVal > bestMetric {
				bestMetric = metricVal
				bestTrial = &trial
				bestParams = params
			}
		}

		select {
		case <-ctx.Done():
			break nextGrid
		default:
		}
	}

	return trials, bestTrial, bestParams, bestMetric
}

func runRandomSearch(ctx context.Context, cfg Config) ([]Trial, *Trial, map[string]any, float64) {
	trials := make([]Trial, 0, cfg.MaxTrials)
	bestTrial := (*Trial)(nil)
	bestParams := map[string]any{}
	bestMetric := -1e9

	for id := 0; id < cfg.MaxTrials; id++ {
		params := randomParams(cfg.ParameterSpace)

		trial := runSingleTrial(ctx, cfg, params, id)
		trials = append(trials, trial)

		if trial.Status == "completed" {
			metricVal := metricValue(trial.Metrics, cfg.MetricTarget)
			if metricVal > bestMetric {
				bestMetric = metricVal
				bestTrial = &trial
				bestParams = params
			}
		}

		select {
		case <-ctx.Done():
			break
		default:
		}
	}

	return trials, bestTrial, bestParams, bestMetric
}

func runSingleTrial(ctx context.Context, cfg Config, params map[string]any, id int) Trial {
	trial := Trial{ID: id, Params: params, Status: "running"}
	start := time.Now()

	// Build strategy from params
	strategy := buildStrategyFromParams(params)

	exchange := cfg.Exchange
	interval := cfg.Interval
	if ex, ok := params["exchange"].(string); ok && ex != "" {
		exchange = ex
	}
	if iv, ok := params["interval"].(string); ok && iv != "" {
		interval = iv
	}

	backtestCfg := backtest.BacktestConfig{
		Symbol:         cfg.Symbol,
		Exchange:       exchange,
		Timeframe:      backtest.Timeframe(interval),
		StartTime:      cfg.StartTime,
		EndTime:        cfg.EndTime,
		InitialCapital: cfg.InitialCapital,
		Strategy:       strategy,
	}

	// Override with params
	if ic, ok := params["initial_capital"].(float64); ok && ic > 0 {
		backtestCfg.InitialCapital = ic
	}
	if leverage, ok := params["leverage"].(int); ok {
		backtestCfg.Leverage = leverage
	}

	result, err := backtest.Run(backtestCfg)
	if err != nil {
		trial.Status = "failed"
		trial.Error = err.Error()
	} else {
		trial.Metrics = result.Metrics
		trial.Status = "completed"
	}

	trial.DurationMs = time.Since(start).Milliseconds()
	return trial
}

func buildStrategyFromParams(params map[string]any) backtest.Strategy {
	fast := 10
	slow := 30
	if fp, ok := params["fast_period"].(float64); ok {
		fast = int(fp)
	}
	if sp, ok := params["slow_period"].(float64); ok {
		slow = int(sp)
	}
	return backtest.NewMACrossStrategy(fast, slow)
}

func metricValue(m backtest.Metrics, target string) float64 {
	switch target {
	case "sharpe_ratio":
		return m.SharpeRatio
	case "total_return", "total_pnl":
		return m.TotalPnL
	case "profit_factor":
		return m.ProfitFactor
	case "win_rate":
		return m.WinRate
	case "max_drawdown":
		return -m.MaxDrawdown // minimize drawdown = maximize negative drawdown
	default:
		return m.SharpeRatio
	}
}

func gridCombinations(params []Parameter) [][]float64 {
	if len(params) == 0 {
		return nil
	}
	if len(params) == 1 {
		p := params[0]
		var vals []float64
		if len(p.Values) > 0 {
			for _, v := range p.Values {
				var f float64
				fmt.Sscanf(v, "%f", &f)
				vals = append(vals, f)
			}
		} else {
			for v := p.Min; v <= p.Max; v += p.Step {
				vals = append(vals, v)
			}
		}
		result := make([][]float64, len(vals))
		for i, v := range vals {
			result[i] = []float64{v}
		}
		return result
	}

	// Recursive: first param + rest
	first := params[0]
	rest := params[1:]
	restCombos := gridCombinations(rest)

	var vals []float64
	if len(first.Values) > 0 {
		for _, v := range first.Values {
			var f float64
			fmt.Sscanf(v, "%f", &f)
			vals = append(vals, f)
		}
	} else {
		for v := first.Min; v <= first.Max; v += first.Step {
			vals = append(vals, v)
		}
	}

	result := make([][]float64, 0)
	for _, v := range vals {
		for _, combo := range restCombos {
			newCombo := make([]float64, 1+len(combo))
			newCombo[0] = v
			copy(newCombo[1:], combo)
			result = append(result, newCombo)
		}
	}
	return result
}

func randomParams(params []Parameter) map[string]any {
	result := map[string]any{}
	for _, p := range params {
		if len(p.Values) > 0 {
			result[p.Name] = p.Values[time.Now().UnixNano()%int64(len(p.Values))]
		} else {
			result[p.Name] = p.Min + float64(time.Now().UnixNano()%int64(p.Max-p.Min+1))
		}
	}
	return result
}

// ParseConfig parses JSON config into experiment.Config
func ParseConfig(data []byte) (*Config, error) {
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse experiment config: %w", err)
	}
	if cfg.Name == "" {
		cfg.Name = "unnamed_experiment"
	}
	if cfg.Mode == "" {
		cfg.Mode = "random"
	}
	if cfg.MetricTarget == "" {
		cfg.MetricTarget = "sharpe_ratio"
	}
	return &cfg, nil
}

// ExampleConfig returns a sample experiment config
func ExampleConfig() string {
	return `{
  "name": "MA Cross Parameter Search",
  "symbol": "BTCUSDT",
  "exchange": "binance",
  "interval": "1h",
  "start_time": "2024-01-01T00:00:00Z",
  "end_time": "2024-12-01T00:00:00Z",
  "initial_capital": 10000,
  "mode": "grid",
  "max_trials": 50,
  "metric_target": "sharpe_ratio",
  "parameter_space": [
    {"name": "fast_period", "type": "int", "min": 5, "max": 30, "step": 5},
    {"name": "slow_period", "type": "int", "min": 20, "max": 100, "step": 10}
  ]
}`
}

// RegisterIndicators registers custom experiment-friendly indicators
func init() {
	// Register any custom indicators with kernel if needed
	_ = kernel.RSI
}