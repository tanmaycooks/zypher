// Package stream provides a buffered writer for Redis Streams output.
// B-08: Fixed missing sync import and data race on buffer — Flush() now
// executes inside the mutex lock to prevent concurrent buffer access.
package stream

import (
	"context"
	"fmt"
	"log/slog"
	"sync" // B-08: was missing from import block
	"time"

	"github.com/redis/go-redis/v9"
)

// Record represents a scraped data record to be written to a Redis Stream.
type Record struct {
	URL         string
	Domain      string
	ContentHash string
	Body        []byte
	StatusCode  int
	FetchedAt   time.Time
	Fields      map[string]string // extracted structured fields
}

// Writer provides buffered writes to a Redis Stream.
// Records are accumulated in a buffer and flushed when the buffer is full
// or when Flush() is called explicitly.
//
// B-08 FIX: The original code used sync.Mutex but didn't import "sync",
// causing a compile error. Additionally, Flush() was called outside the
// mutex lock in Write(), creating a race condition where concurrent
// goroutines could corrupt the buffer during flush.
type Writer struct {
	mu         sync.Mutex // B-08: sync import was missing
	client     *redis.Client
	streamKey  string
	buffer     []Record
	bufferSize int
	logger     *slog.Logger
}

// NewWriter creates a new buffered stream writer.
func NewWriter(client *redis.Client, streamKey string, bufferSize int, logger *slog.Logger) *Writer {
	if logger == nil {
		logger = slog.Default()
	}
	return &Writer{
		client:     client,
		streamKey:  streamKey,
		buffer:     make([]Record, 0, bufferSize),
		bufferSize: bufferSize,
		logger:     logger,
	}
}

// Write adds a record to the buffer and flushes if the buffer is full.
// B-08 FIX: Flush is called INSIDE the mutex lock to prevent race conditions.
func (w *Writer) Write(ctx context.Context, record Record) error {
	w.mu.Lock()
	w.buffer = append(w.buffer, record)

	if len(w.buffer) >= w.bufferSize {
		// B-08 FIX: Flush inside the lock — prevents concurrent buffer access
		err := w.flushLocked(ctx)
		w.mu.Unlock()
		return err
	}

	w.mu.Unlock()
	return nil
}

// Flush writes all buffered records to the Redis Stream.
// Thread-safe — acquires the mutex before flushing.
func (w *Writer) Flush(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.flushLocked(ctx)
}

// flushLocked performs the actual flush. Must be called with mu held.
func (w *Writer) flushLocked(ctx context.Context) error {
	if len(w.buffer) == 0 {
		return nil
	}

	pipe := w.client.Pipeline()
	for _, record := range w.buffer {
		fields := map[string]interface{}{
			"url":          record.URL,
			"domain":       record.Domain,
			"content_hash": record.ContentHash,
			"status_code":  record.StatusCode,
			"fetched_at":   record.FetchedAt.Unix(),
			"body_size":    len(record.Body),
		}
		// Add extracted fields
		for k, v := range record.Fields {
			fields["field_"+k] = v
		}

		pipe.XAdd(ctx, &redis.XAddArgs{
			Stream: w.streamKey,
			Values: fields,
		})
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		w.logger.Error("failed to flush stream buffer",
			"records", len(w.buffer), "error", err)
		return fmt.Errorf("stream flush failed: %w", err)
	}

	w.logger.Info("stream buffer flushed",
		"records", len(w.buffer), "stream", w.streamKey)

	// Reset buffer
	w.buffer = w.buffer[:0]
	return nil
}

// Len returns the number of records currently in the buffer.
func (w *Writer) Len() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.buffer)
}
