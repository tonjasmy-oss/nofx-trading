package backtest

import (
	"context"
	"fmt"
	"sync"
	"time"

	"nofx/provider/coinank"
	"nofx/provider/coinank/coinank_api"
	"nofx/provider/coinank/coinank_enum"
)

// KlineCache provides in-memory caching of kline data.
type KlineCache struct {
	store   map[string]*cacheEntry
	lock    sync.RWMutex
	maxSize int
	ttl     time.Duration
}

type cacheEntry struct {
	data    []KlineBar
	expires time.Time
}

// NewKlineCache creates a new cache with the given max size and default TTL.
func NewKlineCache(maxSize int, defaultTTL time.Duration) *KlineCache {
	return &KlineCache{
		store:   make(map[string]*cacheEntry),
		maxSize: maxSize,
		ttl:     defaultTTL,
	}
}

func cacheKey(exchange, symbol, tf string) string {
	return fmt.Sprintf("%s:%s:%s", exchange, symbol, tf)
}

// Get returns cached klines if available and not expired.
func (c *KlineCache) Get(exchange, symbol, timeframe string) ([]KlineBar, bool) {
	key := cacheKey(exchange, symbol, timeframe)
	c.lock.RLock()
	entry, ok := c.store[key]
	c.lock.RUnlock()
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.expires) {
		c.lock.Lock()
		delete(c.store, key)
		c.lock.Unlock()
		return nil, false
	}
	result := make([]KlineBar, len(entry.data))
	copy(result, entry.data)
	return result, true
}

// Put stores klines in the cache.
func (c *KlineCache) Put(exchange, symbol, timeframe string, klines []KlineBar) {
	key := cacheKey(exchange, symbol, timeframe)
	ttl := c.ttlForTimeframe(timeframe)
	c.lock.Lock()
	defer c.lock.Unlock()
	if len(c.store) >= c.maxSize {
		c.evictOldest()
	}
	data := make([]KlineBar, len(klines))
	copy(data, klines)
	c.store[key] = &cacheEntry{data: data, expires: time.Now().Add(ttl)}
}

func (c *KlineCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time
	first := true
	for k, v := range c.store {
		if first || v.expires.Before(oldestTime) {
			oldestKey = k
			oldestTime = v.expires
			first = false
		}
	}
	if oldestKey != "" {
		delete(c.store, oldestKey)
	}
}

func (c *KlineCache) ttlForTimeframe(tf string) time.Duration {
	switch tf {
	case "1m", "3m", "5m", "15m", "30m":
		return 5 * time.Minute
	default:
		return 30 * time.Minute
	}
}

// FetchKlines fetches klines from the data source, using cache when possible.
func (c *KlineCache) FetchKlines(symbol, exchange, interval string, startTime, endTime time.Time) ([]KlineBar, error) {
	cached, ok := c.Get(exchange, symbol, interval)
	if ok && len(cached) > 0 {
		return c.filterByTimeRange(cached, startTime, endTime), nil
	}

	ci := coinank_enum.Interval(interval)
	ce := c.coinankExchange(exchange)

	ctx := context.Background()
	ts := endTime.UnixMilli()
	size := c.estimateSize(startTime, endTime, interval)

	klines, err := coinank_api.Kline(ctx, symbol, ce, ts, coinank_enum.To, size, ci)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch klines from %s/%s: %w", exchange, symbol, err)
	}

	bars := c.convertKlineResults(klines)
	c.Put(exchange, symbol, interval, bars)
	return c.filterByTimeRange(bars, startTime, endTime), nil
}

func (c *KlineCache) estimateSize(start, end time.Time, interval string) int {
	duration := end.Sub(start)
	secPerBar := TimeframeSeconds[Timeframe(interval)]
	if secPerBar == 0 {
		secPerBar = 3600
	}
	count := int(duration.Seconds())/secPerBar + 1
	if count > 1000 {
		return 1000
	}
	if count < 100 {
		return 100
	}
	return count
}

func (c *KlineCache) filterByTimeRange(bars []KlineBar, start, end time.Time) []KlineBar {
	startMs := start.UnixMilli()
	endMs := end.UnixMilli()
	result := make([]KlineBar, 0, len(bars))
	for _, b := range bars {
		if b.Time >= startMs && b.Time <= endMs {
			result = append(result, b)
		}
	}
	return result
}

func (c *KlineCache) coinankExchange(exchange string) coinank_enum.Exchange {
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

func (c *KlineCache) convertKlineResults(kd []coinank.KlineResult) []KlineBar {
	if len(kd) == 0 {
		return nil
	}
	// CoinAnk returns newest first; reverse for chronological order
	bars := make([]KlineBar, len(kd))
	for i, item := range kd {
		rev := len(kd) - 1 - i
		bars[rev] = KlineBar{
			Time:   item.StartTime,
			Open:   item.Open,
			High:   item.High,
			Low:    item.Low,
			Close:  item.Close,
			Volume: item.Volume,
		}
	}
	return bars
}

// GlobalCache is the shared global cache instance.
var GlobalCache = NewKlineCache(64, 10*time.Minute)

// FetchKlinesGlobal is a convenience wrapper using the global cache.
func FetchKlinesGlobal(symbol, exchange, interval string, startTime, endTime time.Time) ([]KlineBar, error) {
	return GlobalCache.FetchKlines(symbol, exchange, interval, startTime, endTime)
}