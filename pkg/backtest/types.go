package backtest

import (
	"fmt"
	"math"
	"time"
)

// Timeframe supported by the backtest engine
type Timeframe string

const (
	TF1m  Timeframe = "1m"
	TF5m  Timeframe = "5m"
	TF15m Timeframe = "15m"
	TF30m Timeframe = "30m"
	TF1h  Timeframe = "1h"
	TF4h  Timeframe = "4h"
	TF1d  Timeframe = "1d"
	TF1w  Timeframe = "1w"
)

// TimeframeSeconds maps timeframe to seconds
var TimeframeSeconds = map[Timeframe]int{
	TF1m:  60,
	TF5m:  300,
	TF15m: 900,
	TF30m: 1800,
	TF1h:  3600,
	TF4h:  14400,
	TF1d:  86400,
	TF1w:  604800,
}

// KlineBar represents a single OHLCV bar
type KlineBar struct {
	Time   int64   // Unix timestamp in milliseconds
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume float64
}

// UnixMillis returns bar time as time.Time
func (b *KlineBar) UnixMillis() time.Time {
	return time.UnixMilli(b.Time)
}

// BarArray is a slice of KlineBar
type BarArray []KlineBar

// TimeframeData holds klines for a single timeframe
type TimeframeData struct {
	Timeframe Timeframe
	Klines    []KlineBar
}

// OHLC returns open, high, low, close arrays as float64 slices
func (d *TimeframeData) OHLC() (o, h, l, c []float64) {
	n := len(d.Klines)
	o = make([]float64, n)
	h = make([]float64, n)
	l = make([]float64, n)
	c = make([]float64, n)
	for i := 0; i < n; i++ {
		o[i] = d.Klines[i].Open
		h[i] = d.Klines[i].High
		l[i] = d.Klines[i].Low
		c[i] = d.Klines[i].Close
	}
	return
}

// VolumeSlice returns volume series
func (d *TimeframeData) VolumeSlice() []float64 {
	v := make([]float64, len(d.Klines))
	for i := 0; i < len(d.Klines); i++ {
		v[i] = d.Klines[i].Volume
	}
	return v
}

// --- Signal Types ---

// SignalType represents the type of trading signal
type SignalType int

const (
	SignalNone      SignalType = iota
	SignalOpenLong
	SignalCloseLong
	SignalOpenShort
	SignalCloseShort
)

// String returns string representation of signal type
func (s SignalType) String() string {
	switch s {
	case SignalOpenLong:
		return "open_long"
	case SignalCloseLong:
		return "close_long"
	case SignalOpenShort:
		return "open_short"
	case SignalCloseShort:
		return "close_short"
	default:
		return "none"
	}
}

// Signal represents a single trading signal
type Signal struct {
	BarIndex int       // Index of the bar that generated this signal
	Type     SignalType
	Price    float64   // Price at which signal was generated
	Reason   string    // Human-readable reason for the signal
}

// --- Position Types ---

// PositionSide represents the side of a position
type PositionSide int

const (
	PositionFlat  PositionSide = iota
	PositionLong
	PositionShort
)

func (s PositionSide) String() string {
	switch s {
	case PositionLong:
		return "long"
	case PositionShort:
		return "short"
	default:
		return "flat"
	}
}

// PositionStatus tracks position state
type PositionStatus int

const (
	PosOpen   PositionStatus = iota
	PosClosed
)

func (s PositionStatus) String() string {
	if s == PosClosed {
		return "closed"
	}
	return "open"
}

// Position represents an active position during backtest
type Position struct {
	Side       PositionSide
	EntryPrice float64
	Quantity   float64
	EntryTime  time.Time
	ExitTime   time.Time
	ExitPrice  float64
	PNL        float64
	Status     PositionStatus
}

// --- Trade Types ---

// Trade represents a completed trade in backtest
type Trade struct {
	EntryTime  time.Time
	ExitTime   time.Time
	Side       PositionSide
	EntryPrice float64
	ExitPrice  float64
	Quantity   float64
	PnL        float64
	PnLPct     float64
	Reason     string // Exit reason: stop_loss, take_profit, signal, end
}

// EquityPoint represents equity curve data point
type EquityPoint struct {
	Time   time.Time
	Equity float64
}

// Metrics holds backtest performance metrics
type Metrics struct {
	TotalTrades        int
	WinningTrades     int
	LosingTrades      int
	WinRate           float64
	TotalPnL          float64
	TotalPnLPct       float64
	AvgWin            float64
	AvgLoss           float64
	ProfitFactor      float64
	MaxDrawdown       float64
	MaxDrawdownPct    float64
	SharpeRatio       float64
	SortinoRatio      float64
	AnnualReturn      float64
	Volatility        float64
	LargestWin        float64
	LargestLoss       float64
	AvgHoldingBars    float64
	MaxConsecutiveWins  int
	MaxConsecutiveLoss int
}

// Result holds the complete backtest result
type Result struct {
	Config      BacktestConfig
	Metrics     Metrics
	Trades      []Trade
	EquityCurve []EquityPoint
	Signals     []Signal
	FinalEquity float64
	RunDuration time.Duration
}

// BacktestConfig holds backtest configuration
type BacktestConfig struct {
	Symbol          string
	Exchange       string
	Timeframe      Timeframe
	StartTime     time.Time
	EndTime       time.Time
	InitialCapital float64
	Leverage      int
	FeeRate       float64
	Slippage      float64
	MinTradeSize  float64
	StopLossPct   float64
	TakeProfitPct float64
	Strategy      Strategy
}

// Strategy defines the entry/exit logic for a backtest.
type Strategy interface {
	OnBar(bar KlineBar) []OrderSignal
}

// OrderSignal is a signal generated by the strategy.
type OrderSignal struct {
	Direction PositionSide // Long or Short
	Size      float64      // Fraction of equity per trade (0.0–1.0)
}

// Engine runs vectorized backtests over a bar series.
type Engine struct {
	config      BacktestConfig
	positions   []Position
	trades      []Trade
	equity      float64
	equityCurve []float64
}

// NewEngine creates a new backtest engine
func NewEngine(cfg BacktestConfig) *Engine {
	return &Engine{
		config:      cfg,
		positions:   make([]Position, 0),
		trades:      make([]Trade, 0),
		equity:      cfg.InitialCapital,
		equityCurve: make([]float64, 0),
	}
}

// Run executes the backtest and returns the result.
func (e *Engine) Run(bars []KlineBar) (*Result, error) {
	if len(bars) == 0 {
		return nil, fmt.Errorf("no bars provided")
	}
	if e.config.InitialCapital <= 0 {
		return nil, fmt.Errorf("initial capital must be positive")
	}

	e.equity = e.config.InitialCapital
	e.equityCurve = make([]float64, 0, len(bars))
	e.positions = make([]Position, 0)
	e.trades = make([]Trade, 0)

	e.equityCurve = append(e.equityCurve, e.equity)

	for _, bar := range bars {
		e.processBar(bar)
		e.updateEquity(bar.Close)
		e.equityCurve = append(e.equityCurve, e.equity)
	}

	e.closeAllPositions(bars[len(bars)-1].Close)

	return e.buildResult(bars), nil
}

func (e *Engine) processBar(bar KlineBar) {
	if e.config.Strategy == nil {
		return
	}
	signals := e.config.Strategy.OnBar(bar)
	for _, sig := range signals {
		if sig.Size <= 0 || sig.Size > 1 {
			continue
		}
		e.executeSignal(sig)
	}
}

func (e *Engine) executeSignal(sig OrderSignal) {
	if e.hasOpenPosition() {
		return
	}
	size := e.equity * sig.Size
	if size < e.config.MinTradeSize {
		return
	}
	e.openPosition(sig.Direction, size, 0)
}

func (e *Engine) hasOpenPosition() bool {
	for _, p := range e.positions {
		if p.Status == PosOpen {
			return true
		}
	}
	return false
}

func (e *Engine) openPosition(dir PositionSide, size, entryPrice float64) {
	feeCost := size * e.config.FeeRate
	e.equity -= feeCost

	pos := Position{
		Side:       dir,
		EntryPrice: entryPrice,
		Quantity:   size,
		EntryTime:  time.Now(),
		Status:     PosOpen,
	}
	e.positions = append(e.positions, pos)
}

func (e *Engine) updateEquity(closePrice float64) {
	for i := range e.positions {
		p := &e.positions[i]
		if p.Status != PosOpen {
			continue
		}
		if p.EntryPrice == 0 {
			p.EntryPrice = closePrice
			continue
		}
		var pnl float64
		if p.Side == PositionLong {
			pnl = (closePrice - p.EntryPrice) / p.EntryPrice * p.Quantity
		} else {
			pnl = (p.EntryPrice - closePrice) / p.EntryPrice * p.Quantity
		}
		e.equity += pnl
		p.PNL = pnl
	}
}

func (e *Engine) closeAllPositions(closePrice float64) {
	for i := range e.positions {
		p := &e.positions[i]
		if p.Status != PosOpen {
			continue
		}
		e.closePosition(p, closePrice)
	}
}

func (e *Engine) closePosition(p *Position, closePrice float64) {
	if p.EntryPrice == 0 {
		p.EntryPrice = closePrice
	}
	var pnl float64
	if p.Side == PositionLong {
		pnl = (closePrice - p.EntryPrice) / p.EntryPrice * p.Quantity
	} else {
		pnl = (p.EntryPrice - closePrice) / p.EntryPrice * p.Quantity
	}
	feeCost := p.Quantity * e.config.FeeRate
	pnl -= feeCost
	p.PNL = pnl
	p.ExitPrice = closePrice
	p.ExitTime = time.Now()
	p.Status = PosClosed
	e.equity += p.Quantity + pnl
	e.trades = append(e.trades, Trade{
		EntryTime:  p.EntryTime,
		ExitTime:   p.ExitTime,
		Side:       p.Side,
		EntryPrice: p.EntryPrice,
		ExitPrice:  closePrice,
		Quantity:   p.Quantity,
		PnL:       pnl,
	})
}

func (e *Engine) buildResult(bars []KlineBar) *Result {
	metrics := e.calcMetrics()
	return &Result{
		Config:      e.config,
		Metrics:     metrics,
		Trades:      e.trades,
		EquityCurve: e.buildEquityCurve(),
		FinalEquity: e.equity,
	}
}

func (e *Engine) buildEquityCurve() []EquityPoint {
	points := make([]EquityPoint, len(e.equityCurve))
	for i, eq := range e.equityCurve {
		points[i] = EquityPoint{Equity: eq}
	}
	return points
}

func (e *Engine) calcMetrics() Metrics {
	if len(e.equityCurve) == 0 {
		return Metrics{}
	}

	var returns []float64
	for i := 1; i < len(e.equityCurve); i++ {
		if e.equityCurve[i-1] > 0 {
			ret := (e.equityCurve[i] - e.equityCurve[i-1]) / e.equityCurve[i-1]
			returns = append(returns, ret)
		}
	}

	sharpe := calcSharpe(returns)
	maxDD := calcMaxDD(e.equityCurve)

	var wins, losses int
	var grossProfit, grossLoss float64
	for _, t := range e.trades {
		if t.PnL > 0 {
			wins++
			grossProfit += t.PnL
		} else {
			losses++
			grossLoss += math.Abs(t.PnL)
		}
	}
	total := wins + losses
	var winRate float64
	if total > 0 {
		winRate = float64(wins) / float64(total)
	}
	var pf float64
	if grossLoss > 0 {
		pf = grossProfit / grossLoss
	} else if grossProfit > 0 {
		pf = grossProfit
	}

	initialEq := e.equityCurve[0]
	finalEq := e.equityCurve[len(e.equityCurve)-1]
	totalRet := (finalEq - initialEq) / initialEq

	return Metrics{
		TotalTrades:     len(e.trades),
		WinningTrades:   wins,
		LosingTrades:    losses,
		WinRate:         winRate,
		TotalPnL:        finalEq - initialEq,
		TotalPnLPct:     totalRet,
		ProfitFactor:    pf,
		MaxDrawdown:     maxDD,
		MaxDrawdownPct:  maxDD * 100,
		SharpeRatio:     sharpe,
		AvgWin:          calcAvgWin(e.trades),
		AvgLoss:         calcAvgLoss(e.trades),
		LargestWin:      calcLargestWin(e.trades),
		LargestLoss:     calcLargestLoss(e.trades),
	}
}

func calcSharpe(returns []float64) float64 {
	if len(returns) < 2 {
		return 0
	}
	var sum, sumSq float64
	for _, r := range returns {
		sum += r
		sumSq += r * r
	}
	n := float64(len(returns))
	mean := sum / n
	variance := sumSq/n - mean*mean
	if variance <= 0 {
		return 0
	}
	stdDev := math.Sqrt(variance)
	if stdDev == 0 {
		return 0
	}
	return (mean / stdDev) * math.Sqrt(252)
}

func calcMaxDD(curve []float64) float64 {
	if len(curve) == 0 {
		return 0
	}
	peak := curve[0]
	maxDD := 0.0
	for _, v := range curve {
		if v > peak {
			peak = v
		}
		dd := (peak - v) / peak
		if dd > maxDD {
			maxDD = dd
		}
	}
	return maxDD
}

func calcAvgWin(trades []Trade) float64 {
	var sum float64
	var count int
	for _, t := range trades {
		if t.PnL > 0 {
			sum += t.PnL
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return sum / float64(count)
}

func calcAvgLoss(trades []Trade) float64 {
	var sum float64
	var count int
	for _, t := range trades {
		if t.PnL < 0 {
			sum += t.PnL
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return sum / float64(count)
}

func calcLargestWin(trades []Trade) float64 {
	var largest float64
	for _, t := range trades {
		if t.PnL > largest {
			largest = t.PnL
		}
	}
	return largest
}

func calcLargestLoss(trades []Trade) float64 {
	var smallest float64
	for _, t := range trades {
		if t.PnL < smallest {
			smallest = t.PnL
		}
	}
	return smallest
}

// RunBacktest is the main entry point
func RunBacktest(cfg BacktestConfig, bars []KlineBar) (*Result, error) {
	return NewEngine(cfg).Run(bars)
}