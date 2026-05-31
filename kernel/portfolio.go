package kernel

// Portfolio holds equity and position tracking
type Portfolio struct {
	InitialCapital float64
	TotalPnL       float64
	Positions      map[string]*PositionSummary
}

// PositionSummary holds position details for portfolio
type PositionSummary struct {
	Symbol       string
	Quantity     float64
	AvgPrice     float64
	CurrentPrice float64
	Side         string // "long" or "short"
	UnrealizedPnL float64
}

// NewPortfolio creates a new portfolio tracker
func NewPortfolio(initialCapital float64) *Portfolio {
	return &Portfolio{
		InitialCapital: initialCapital,
		Positions:      make(map[string]*PositionSummary),
	}
}

// UpdatePosition updates or adds a position
func (p *Portfolio) UpdatePosition(symbol string, quantity, avgPrice, currentPrice float64, side string) {
	if quantity == 0 {
		// Remove position if quantity is 0
		delete(p.Positions, symbol)
		return
	}

	pnl := p.calculatePositionPnL(symbol, quantity, avgPrice, currentPrice, side)

	p.Positions[symbol] = &PositionSummary{
		Symbol:        symbol,
		Quantity:      quantity,
		AvgPrice:      avgPrice,
		CurrentPrice:  currentPrice,
		Side:          side,
		UnrealizedPnL: pnl,
	}
}

func (p *Portfolio) calculatePositionPnL(symbol string, quantity, avgPrice, currentPrice float64, side string) float64 {
	if side == "long" {
		return (currentPrice - avgPrice) * quantity
	}
	// short
	return (avgPrice - currentPrice) * quantity
}

// GetEquity returns current equity (initial capital + total PnL)
func (p *Portfolio) GetEquity() float64 {
	return p.InitialCapital + p.TotalPnL
}

// GetTotalUnrealizedPnL calculates total unrealized PnL from positions
func (p *Portfolio) GetTotalUnrealizedPnL() float64 {
	var total float64
	for _, pos := range p.Positions {
		total += pos.UnrealizedPnL
	}
	return total
}

// GetAvailableBalance calculates available balance
// Available = Equity - used margin
func (p *Portfolio) GetAvailableBalance() float64 {
	equity := p.GetEquity()
	var usedMargin float64
	for _, pos := range p.Positions {
		usedMargin += pos.AvgPrice * pos.Quantity
	}
	return equity - usedMargin
}

// GetPositionsSummary returns a summary of all positions
func (p *Portfolio) GetPositionsSummary() []PositionSummary {
	result := make([]PositionSummary, 0, len(p.Positions))
	for _, pos := range p.Positions {
		result = append(result, *pos)
	}
	return result
}

// GetPositionCount returns the number of active positions
func (p *Portfolio) GetPositionCount() int {
	return len(p.Positions)
}

// RecalculatePositions recalculates PnL for all positions based on current prices
func (p *Portfolio) RecalculatePositions() {
	for symbol, pos := range p.Positions {
		pnl := p.calculatePositionPnL(symbol, pos.Quantity, pos.AvgPrice, pos.CurrentPrice, pos.Side)
		pos.UnrealizedPnL = pnl
	}
}

// ClearPositions removes all positions
func (p *Portfolio) ClearPositions() {
	p.Positions = make(map[string]*PositionSummary)
}