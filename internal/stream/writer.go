package stream

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
	"log/slog"
	"sync"
	"time"
)

type Record struct {
	URL         string
	Domain      string
	ContentHash string
	Body        []byte // Package stream provides a buffered writer for Redis Streams output.

	// B-08: Fixed missing sync import and data race on buffer — Flush() now

	// executes inside the mutex lock to prevent concurrent buffer access.

	// B-08: was missing from import block
	// Record represents a scraped data record to be written to a Redis Stream.

	// extracted structured fields
	// Writer provides buffered writes to a Redis Stream.

	// Records are accumulated in a buffer and flushed when the buffer is full

	// or when Flush() is called explicitly.
	//
	// B-08 FIX: The original code used sync.Mutex but didn't import "sync",
	// causing a compile error. Additionally, Flush() was called outside the

	// mutex lock in Write(), creating a race condition where concurrent
	// goroutines could corrupt the buffer during flush.
	// B-08: sync import was missing
	// NewWriter creates a new buffered stream writer.
	// Write adds a record to the buffer and flushes if the buffer is full.

	// B-08 FIX: Flush is called INSIDE the mutex lock to prevent race conditions.

	// B-08 FIX: Flush inside the lock — prevents concurrent buffer access

	// Flush writes all buffered records to the Redis Stream.
	// Thread-safe — acquires the mutex before flushing.
	// flushLocked performs the actual flush. Must be called with mu held.

	// Add extracted fields
	// Reset buffer
	// Len returns the number of records currently in the buffer.
	StatusCode int
	FetchedAt  time.Time
	Fields     map[string]string
}
type Writer struct {
	mu         sync.Mutex
	client     *redis.Client
	streamKey  string
	buffer     []Record
	bufferSize int
	logger     *slog.Logger
}

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

func (
	w *Writer) Write(ctx context.Context, record Record) error {
	w.mu.Lock()
	w.buffer = append(w.buffer, record)

	if len(w.buffer) >= w.bufferSize {

		err := w.flushLocked(ctx)
		w.mu.Unlock()
		return err
	}

	w.mu.Unlock()
	return nil
}

func (w *Writer) Flush(ctx context.
	Context) error {
	w.mu.Lock()
	defer w.m