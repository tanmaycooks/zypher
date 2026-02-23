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

	// emaAlpha controls the smoothing factor of 