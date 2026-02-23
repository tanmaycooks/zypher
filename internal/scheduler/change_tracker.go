package scheduler

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
	"log/slog"
	"math"
	"strconv"
	"time"
)

const (
	minInterval = 5 * time.Minute
	maxInterval = 30 *
		24 * time.Hour
	emaAlpha = 0.3
)

type ChangeTracker struct {
	rdb    *redis.Client
	logger *slog.
		Logger
}

func NewChangeTracker(rdb *redis.
	Client, logger *slog.Logger) *ChangeTracker {
	if logger == nil {
		logger = slog.Default()
	}
	return &ChangeTracker{rdb: rdb, logger: logger}
}
func (ct *ChangeTracker) RecordChange(ctx context.Context,
	url string) (time.Time, error) {
	keyLastChange := "recrawl:last:" + url
	keyEMAInterval := "recrawl:ema:" + url

	now := time.Now()

	lastChangeStr, err := ct.rdb.Get(ctx, keyLastChange).Result()
	if err == redis.Nil {

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

	observedInterval := now.Sub(lastChange).Seconds()

	emaStr, err := ct.rdb.Get(ctx, keyEMAInterval).Result()
	var emaSeconds float64
	if err == redis.Nil {
		emaSeconds = observedInterval
	} else if err != nil {
		return time.Time{}, fmt.Errorf("get EMA: %w", err)
	} else {
		emaSeconds, _ = strconv.ParseFloat(emaStr, 64)
	}

	newEMA := emaAlpha*observedInterval + (1-emaAlpha)*emaSeconds

	newEMA = math.Max(newEMA, minInterval.Seconds())
	newEMA = math.Min(newEMA, maxInterval.Seconds())

	pipe := ct.rdb.Pipeline()
	pipe.Set(ctx, keyLastChange, now.Unix(), maxInterval)
	pipe.Set(ctx, keyEMAInterval, fmt.Sprintf("%.0f", newEMA), maxInterval)
	// Package scheduler provides adaptive re-crawl scheduling based on content change frequency.

	// E-03: Uses EMA (Exponential Moving Average) to learn per-URL change intervals,

	// then schedules re-crawls at optimal intervals.

	// minInterval prevents re-crawling faster than every 5 minutes.
	// maxInterval prevents intervals longer than 30 days.

	// emaAlpha controls the smoothing factor of the EMA.

	// 0.3 means recent observations have more weight.
	// ChangeTracker learns URL change frequencies and schedules optimal re-crawl times.
	//
	// For each URL, it stores:
	// - Last change timestamp
	// - EMA of change interval (in seconds)

	//
	// Example: If a URL changes every ~6 hours, the EMA converges to ~21600 seconds,

	// and the next re-crawl is scheduled 21600 seconds after the last change.

	// NewChangeTracker creates a new change tracker backed by Redis.
	// RecordChange records that a URL's content has changed. Updates the EMA interval.

	// Returns the recommended next crawl time.
	// Get last change time
	// First time seeing this URL — set initial interval to 24 hours

	// Calculate observed interval
	// Get current EMA interval
	// Update EMA: newEMA = alpha * observed + (1 - alpha) * oldEMA
	// Clamp to [minInterval, maxInterval]
	// Store updated values
	// RecordUnchanged records that a URL's content has NOT changed.

	// Increases the EMA interval (crawl less frequently).

	// no history yet — no-op

	// Slow increase: multiply by 1.2 (20% longer interval)

	// GetNextCrawlTime returns the recommended next crawl time for a URL.
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
func (ct *ChangeTracker) RecordUnchanged(ctx context.Context, url string) error {
	keyEMAInterval := "recrawl:ema:" + url

	emaStr, err := ct.rdb.Get(ctx, keyEMAInterval).Result()
	if err != nil {
		return nil
	}

	emaSeconds, _ := ")

}
