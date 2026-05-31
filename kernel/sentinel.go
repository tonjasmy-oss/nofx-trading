package kernel

import (
	"log/slog"
	"time"
)

// SentinelConfig configures the sentinel rule engine
type SentinelConfig struct {
	Enabled            bool    // Enable/disable sentinel
	Mode               string  // "block" (default) or "alert"
	MaxPositionPct     float64 // Max total position ratio (default 0.3 = 30%)
	MaxTradesPerSymbol int     // Max trades per symbol in lookback (default 3)
	FreqLookbackMins   int     // Lookback window for frequency check (default 60)
	RequireEMAAlign    bool    // Require signal direction to align with EMA (default true)
	MinVolumeRatio     float64 // Minimum volume ratio (default 0.5)
	MaxConsecutiveLoss int     // Max consecutive losses before cooldown (default 3)
	MaxDrawdownPct     float64 // Max drawdown before rejecting trades (default 0.15)
	MinPositionHours   float64 // Minimum position holding time (default 0, disabled)
	MaxSingleExposure  float64 // Max exposure per single symbol (default 0.2)
	SentinelBlocksOnAlert bool // In alert mode, block trades anyway (default false)
}

// SentinelRule represents a single sentinel rule
type SentinelRule struct {
	Name  string
	Check func(signal *TradeSignal, market *MarketData, state *SentinelState, cfg *SentinelConfig) *RuleResult
}

// RuleResult represents the result of a rule check
type RuleResult struct {
	Passed bool
	Reason string
}

// SentinelState holds state for sentinel rule evaluation
type SentinelState struct {
	Equity          float64
	ConsecutiveLoss int
	TotalPositionPct float64
	TradeHistory    []TradeRef
}

// TradeRef is a reference to a trade in history
type TradeRef struct {
	Symbol    string
	Timestamp int64
	PnL       float64
}

// SentinelRuleEngine is the sentinel rule evaluation engine
type SentinelRuleEngine struct {
	config *SentinelConfig
	rules  []SentinelRule
	logger *slog.Logger
}

// NewSentinelRuleEngine creates a new sentinel rule engine
func NewSentinelRuleEngine(cfg *SentinelConfig, logger *slog.Logger) *SentinelRuleEngine {
	if cfg == nil {
		cfg = &SentinelConfig{
			MaxPositionPct:     0.3,
			MaxTradesPerSymbol: 3,
			FreqLookbackMins:   60,
			RequireEMAAlign:    true,
			MinVolumeRatio:     0.5,
			MaxConsecutiveLoss: 3,
			MaxDrawdownPct:    0.15,
			MinPositionHours:   0,
			MaxSingleExposure:  0.2,
			Mode:               "block",
		}
	}
	if logger == nil {
		logger = slog.Default()
	}

	engine := &SentinelRuleEngine{
		config: cfg,
		rules:  make([]SentinelRule, 0),
		logger: logger,
	}

	// Register all 8 rules
	engine.registerRules()
	return engine
}

func (s *SentinelRuleEngine) registerRules() {
	s.rules = append(s.rules, SentinelRule{Name: "R1_PositionLimit", Check: s.rulePositionLimit})
	s.rules = append(s.rules, SentinelRule{Name: "R2_TradeFrequency", Check: s.ruleTradeFrequency})
	s.rules = append(s.rules, SentinelRule{Name: "R3_EMATrendFilter", Check: s.ruleEMATrendFilter})
	s.rules = append(s.rules, SentinelRule{Name: "R4_VolumeFilter", Check: s.ruleVolumeFilter})
	s.rules = append(s.rules, SentinelRule{Name: "R5_PriceLimit", Check: s.rulePriceLimit})
	s.rules = append(s.rules, SentinelRule{Name: "R6_ConsecutiveLoss", Check: s.ruleConsecutiveLoss})
	s.rules = append(s.rules, SentinelRule{Name: "R7_PositionAge", Check: s.rulePositionAge})
	s.rules = append(s.rules, SentinelRule{Name: "R8_ExposureLimit", Check: s.ruleExposureLimit})
}

// RunSentinelRules runs all sentinel rules against a signal
// Returns (blocked bool, reason string)
func (s *SentinelRuleEngine) RunSentinelRules(signal *TradeSignal, market *MarketData, state *SentinelState) (bool, string) {
	if s.config == nil || !s.config.Enabled {
		return false, ""
	}

	var blockedReasons []string

	for _, rule := range s.rules {
		result := rule.Check(signal, market, state, s.config)
		if !result.Passed {
			blockedReasons = append(blockedReasons, result.Reason)
			s.logger.Debug("Sentinel rule triggered",
				"rule", rule.Name,
				"reason", result.Reason,
				"symbol", signal.Symbol)
		}
	}

	if len(blockedReasons) == 0 {
		return false, ""
	}

	reason := joinReasons(blockedReasons)

	if s.config.Mode == "alert" && !s.config.SentinelBlocksOnAlert {
		// Alert mode: log but don't block
		s.logger.Warn("Sentinel alert (not blocking)",
			"symbol", signal.Symbol,
			"reasons", reason)
		return false, reason
	}

	// Block mode or alert mode with blocks enabled
	return true, reason
}

// Join reasons with semicolon separator
func joinReasons(reasons []string) string {
	if len(reasons) == 0 {
		return ""
	}
	result := reasons[0]
	for i := 1; i < len(reasons); i++ {
		result += "; " + reasons[i]
	}
	return result
}

// R1: Position limit - total position cannot exceed max
func (s *SentinelRuleEngine) rulePositionLimit(signal *TradeSignal, market *MarketData, state *SentinelState, cfg *SentinelConfig) *RuleResult {
	maxPosPct := cfg.MaxPositionPct
	if maxPosPct <= 0 {
		maxPosPct = 0.3 // Default 30%
	}

	signalRatio := 0.1 // Default signal ratio
	if signal != nil && signal.PositionRatio > 0 {
		signalRatio = signal.PositionRatio
	}

	currentRatio := 0.0
	if state != nil {
		currentRatio = state.TotalPositionPct
	}

	if currentRatio+signalRatio > maxPosPct {
		return &RuleResult{
			Passed: false,
			Reason: "R1_仓位超限",
		}
	}
	return &RuleResult{Passed: true}
}

// R2: Trade frequency - same symbol cannot be traded too frequently
func (s *SentinelRuleEngine) ruleTradeFrequency(signal *TradeSignal, market *MarketData, state *SentinelState, cfg *SentinelConfig) *RuleResult {
	if signal == nil || state == nil || len(state.TradeHistory) == 0 {
		return &RuleResult{Passed: true}
	}

	maxFreq := cfg.MaxTradesPerSymbol
	if maxFreq <= 0 {
		maxFreq = 3
	}

	lookback := cfg.FreqLookbackMins
	if lookback <= 0 {
		lookback = 60
	}

	lookbackSeconds := int64(lookback * 60)
	now := time.Now().Unix()

	symbol := signal.Symbol
	count := 0

	for i := len(state.TradeHistory) - 1; i >= 0; i-- {
		trade := state.TradeHistory[i]
		if trade.Symbol != symbol {
			continue
		}
		if now-trade.Timestamp < lookbackSeconds {
			count++
		} else {
			break // Past lookback window, stop counting
		}
	}

	if count >= maxFreq {
		return &RuleResult{
			Passed: false,
			Reason: "R2_交易频繁",
		}
	}
	return &RuleResult{Passed: true}
}

// R3: EMA trend filter - signal direction must align with EMA
func (s *SentinelRuleEngine) ruleEMATrendFilter(signal *TradeSignal, market *MarketData, state *SentinelState, cfg *SentinelConfig) *RuleResult {
	if !cfg.RequireEMAAlign {
		return &RuleResult{Passed: true}
	}

	if market == nil || signal == nil {
		return &RuleResult{Passed: true} // No market data to check
	}

	emaFast := market.EMAFast
	emaSlow := market.EMASlow

	if emaFast <= 0 || emaSlow <= 0 {
		return &RuleResult{Passed: true} // No EMA data
	}

	signalSide := "long"
	if signal.Action == "open_short" || signal.Action == "close_long" {
		signalSide = "short"
	}

	// EMA trend: fast > slow = uptrend, fast < slow = downtrend
	isUptrend := emaFast > emaSlow

	if signalSide == "long" && !isUptrend {
		return &RuleResult{
			Passed: false,
			Reason: "R3_EMA趋势看跌",
		}
	}
	if signalSide == "short" && isUptrend {
		return &RuleResult{
			Passed: false,
			Reason: "R3_EMA趋势看涨",
		}
	}

	return &RuleResult{Passed: true}
}

// R4: Volume filter - volume ratio must meet minimum
func (s *SentinelRuleEngine) ruleVolumeFilter(signal *TradeSignal, market *MarketData, state *SentinelState, cfg *SentinelConfig) *RuleResult {
	if market == nil {
		return &RuleResult{Passed: true}
	}

	minVolRatio := cfg.MinVolumeRatio
	if minVolRatio <= 0 {
		minVolRatio = 0.5
	}

	volRatio := market.VolumeRatio
	if volRatio <= 0 {
		volRatio = 1.0 // Default to pass if unknown
	}

	if volRatio < minVolRatio {
		return &RuleResult{
			Passed: false,
			Reason: "R4_成交量萎缩",
		}
	}
	return &RuleResult{Passed: true}
}

// R5: Price limit - cannot trade when near limit up/down
func (s *SentinelRuleEngine) rulePriceLimit(signal *TradeSignal, market *MarketData, state *SentinelState, cfg *SentinelConfig) *RuleResult {
	if market == nil {
		return &RuleResult{Passed: true}
	}

	if market.IsLimitUp || market.IsLimitDown {
		return &RuleResult{
			Passed: false,
			Reason: "R5_涨跌停",
		}
	}
	return &RuleResult{Passed: true}
}

// R6: Consecutive loss cooldown
func (s *SentinelRuleEngine) ruleConsecutiveLoss(signal *TradeSignal, market *MarketData, state *SentinelState, cfg *SentinelConfig) *RuleResult {
	if state == nil {
		return &RuleResult{Passed: true}
	}

	maxConsecutive := cfg.MaxConsecutiveLoss
	if maxConsecutive <= 0 {
		maxConsecutive = 3
	}

	if state.ConsecutiveLoss >= maxConsecutive {
		return &RuleResult{
			Passed: false,
			Reason: "R6_连错超限",
		}
	}
	return &RuleResult{Passed: true}
}

// R7: Position age check - new positions must hold minimum time (R7b from Python)
func (s *SentinelRuleEngine) rulePositionAge(signal *TradeSignal, market *MarketData, state *SentinelState, cfg *SentinelConfig) *RuleResult {
	minHours := cfg.MinPositionHours
	if minHours <= 0 {
		return &RuleResult{Passed: true} // Disabled
	}

	if market == nil || market.PositionAgeHours <= 0 {
		return &RuleResult{Passed: true} // No position or unknown
	}

	if market.PositionAgeHours < minHours {
		return &RuleResult{
			Passed: false,
			Reason: "R7b_持仓过新",
		}
	}
	return &RuleResult{Passed: true}
}

// R8: Single symbol exposure limit
func (s *SentinelRuleEngine) ruleExposureLimit(signal *TradeSignal, market *MarketData, state *SentinelState, cfg *SentinelConfig) *RuleResult {
	if signal == nil || state == nil {
		return &RuleResult{Passed: true}
	}

	maxExposure := cfg.MaxSingleExposure
	if maxExposure <= 0 {
		maxExposure = 0.2
	}

	signalRatio := signal.PositionRatio
	if signalRatio <= 0 {
		signalRatio = 0.1
	}

	// Track per-symbol exposure (in a real implementation, you'd track this per symbol)
	// For simplicity, we use the signal's symbol exposure ratio
	currentExp := 0.0 // Would need to be tracked per symbol in practice

	if currentExp+signalRatio > maxExposure {
		return &RuleResult{
			Passed: false,
			Reason: "R8_单标的暴露超限",
		}
	}
	return &RuleResult{Passed: true}
}

// TradeSignal represents a trading signal for sentinel evaluation
type TradeSignal struct {
	Symbol       string
	Action       string // "open_long", "open_short", "close_long", "close_short", "hold"
	Side         string // "long" or "short"
	PositionRatio float64
}

// MarketData represents market data for sentinel evaluation
type MarketData struct {
	EMAFast        float64 // Fast EMA value
	EMASlow        float64 // Slow EMA value
	VolumeRatio    float64 // Volume ratio
	IsLimitUp      bool    // Is limit up
	IsLimitDown    bool    // Is limit down
	PositionAgeHours float64 // Hours since position opened
}

// IsEnabled returns true if sentinel is enabled
func (cfg *SentinelConfig) IsEnabled() bool {
	return cfg != nil && cfg.Enabled
}

// IsAlertMode returns true if in alert mode (notify but don't block)
func (cfg *SentinelConfig) IsAlertMode() bool {
	return cfg != nil && cfg.Mode == "alert"
}