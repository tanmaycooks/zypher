// Package scheduler provides adaptive re-crawl scheduling based on content change frequency.
// E-03: Uses EMA (Exponential Moving Average) to learn per-URL change intervals,
// then schedules re-crawls at optimal intervals.
package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// minInterval prevents re-crawling faster than every 5 minutes.
	minInterval = 5 * time.Minute
	// maxInterval prevents intervals longer than 30 days.
	maxInterval = 30 * 24 * time.Hour
	// emaAlpha controls the smoothing factor of the EMA.
	// 0.3 means recent observations have more weight.
	emaAlpha = 0.3
)

// ChangeTracker learns URL change frequencies and schedules optimal re-crawl times.
//
// For each URL, it stores:
// - Last change timestamp
// - EMA of change interval (in seconds)
//
// Example: If a URL changes every ~6 hours, the EMA converges to ~21600 seconds,
// and the next re-crawl is scheduled 21600 seconds after the last change.
type ChangeTracker struct {
	rdb    *redis.Client
	logger *slog.Logger
}

// NewChangeTracker creates a new change tracker backed by Redis.
func NewChangeTracker(rdb *redis.Client, logger *slog.Logger) *ChangeTracker {
	if logger == nil {
		logger = slog.Default()
	}
	return &ChangeTracker{rdb: rdb, logger: logger}
}

// RecordChange records that a URL's content has changed. Updates the EMA interval.
// Returns the recommended next crawl time.
func (ct *ChangeTracker) RecordChange(ctx context.Context, url string) (time.Time, error) {
	keyLastChange := "recrawl:last:" + url
	keyEMAInterval := "recrawl:ema:" + url

	now := time.Now()

	// Get last change time
	lastChangeStr, err := ct.rdb.Get(ctx, keyLastChange).Result()
	if err == redis.Nil {
		// First time seeing this URL — set initial interval to 24 hours
		pipe := ct.rdb.Pipeline()
		pipe.Set(ctx, keyLastChange, now.Unix(), maxInterval)
		pipe.Set(ctx, keyEMAInterval, fmt.Sprintf("%.0f", (24*time.Hour).Seconds()), maxInterval)
		_, err := pipe.Exec(ctx)
		if err != nil {
			return time.Time{}, fmt.Errorf("pipeline exec: %w", err)
		}
		return now.Add(24 * time.Hour), nil
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("get last change: %w", err)
	}

	lastChangeUnix, _ := strconv.ParseInt(lastChangeStr, 10, 64)
	lastChange := time.Unix(lastChangeUnix, 0)

	// Calculate observed interval
	observedInterval := now.Sub(lastChange).Seconds()

	// Get current EMA interval
	emaStr, err := ct.rdb.Get(ctx, keyEMAInterval).Result()
	var emaSeconds float64
	if err == redis.Nil {
		emaSeconds = observedInterval
	} else if err != nil {
		return time.Time{}, fmt.Errorf("get EMA: %w", err)
	} else {
		emaSeconds, _ = strconv.ParseFloat(emaStr, 64)
	}

	// Update EMA: newEMA = alpha * observed + (1 - alpha) * oldEMA
	newEMA := emaAlpha*observedInterval + (1-emaAlpha)*emaSeconds

	// Clamp to [minInterval, maxInterval]
	newEMA = math.Max(newEMA, minInterval.Seconds())
	newEMA = math.Min(newEMA, maxInterval.Seconds())

	// Store updated values
	pipe := ct.rdb.Pipeline()
	pipe.Set(ctx, keyLastChange, now.Unix(), maxInterval)
	pipe.Set(ctx, keyEMAInterval, fmt.Sprintf("%.0f", newEMA), maxInterval)
	_, err = pipe.Exec(ctx)
	if err != nil {
		return time.Time{}, fmt.Errorf("update pipeline: %w", err)
	}

	nextCrawl := now.Add(time.Duration(newEMA) * time.Second)
	ct.logger.Debug("change recorded",
		"url", url,
		"observed_interval_h", observedInterval/3600,
		"ema_interval_h", newEMA/3600,
		"next_crawl", nextCrawl)

	return nextCrawl, nil
}

// RecordUnchanged records that a URL's content has NOT changed.
// Increases the EMA interval (crawl less frequently).
func (ct *ChangeTracker) RecordUnchanged(ctx context.Context, url string) error {
	keyEMAInterval := "recrawl:ema:" + url

	emaStr, err := ct.rdb.Get(ctx, keyEMAInterval).Result()
	if err != nil {
		return nil // no history yet — no-op
	}

	emaSeconds, _ := strconv.ParseFloat(emaStr, 64)

	// Slow increase: multiply by 1.2 (20% longer interval)
	newEMA := emaSeconds * 1.2
	newEMA = math.Min(newEMA, maxInterval.Seconds())

	return ct.rdb.Set(ctx, keyEMAInterval, fmt.Sprintf("%.0f", newEMA), maxInterval).Err()
}

// GetNextCrawlTime returns the recommended next crawl time for a URL.
func (ct *ChangeTracker) GetNextCrawlTime(ctx context.Context, url string) (time.Time, error) {
	keyLastChange := "recrawl:last:" + url
	keyEMAInterval := "recrawl:ema:" + url

	lastChangeStr, err := ct.rdb.Get(ctx, keyLastChange).Result()
	if err != nil {
		return time.Time{}, err
	}

	emaStr, err := ct.rdb.Get(ctx, keyEMAInterval).Result()
	if err != nil {
		return time.Time{}, err
	}

	lastChangeUnix, _ := strconv.ParseInt(lastChangeStr, 10, 64)
	emaSeconds, _ := strconv.ParseFloat(emaStr, 64)

	nextCrawl := time.Unix(lastChangeUnix, 0).Add(time.Duration(emaSeconds) * time.Second)
	return nextCrawl, nil
}
