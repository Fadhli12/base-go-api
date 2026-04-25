package logger

import (
	"context"
	"fmt"
	"os"
	"sync"
)

// Writer is the interface for log output destinations.
//
// CONTRACT:
//   - Write must be goroutine-safe
//   - Write must not block indefinitely (implement timeout if needed)
//   - Close must flush any buffered data
//   - Close may be called multiple times safely
//   - Write may be called after Close but should return error
//
// Usage:
//
//	writer, err := logger.NewStdoutWriter("json")
//	if err != nil {
//	    return err
//	}
//	defer writer.Close()
//
//	entry := logger.LogEntry{...}
//	if err := writer.Write(ctx, entry); err != nil {
//	    // Handle error
//	}
type Writer interface {
	// Write outputs a log entry to the destination.
	// Returns error if write fails or destination is closed.
	// CONTRACT: Goroutine-safe, non-blocking
	Write(ctx context.Context, entry LogEntry) error

	// Close flushes any buffered data and releases resources.
	// Returns error if close fails, nil on success.
	// CONTRACT: Safe to call multiple times
	Close() error
}

// WriterFunc is an adapter to allow the use of ordinary functions as Writers.
//
// Example:
//
//	w := WriterFunc(func(ctx context.Context, entry LogEntry) error {
//	    fmt.Println(entry.Message)
//	    return nil
//	})
type WriterFunc func(ctx context.Context, entry LogEntry) error

// Write calls f(ctx, entry) and returns the result.
func (f WriterFunc) Write(ctx context.Context, entry LogEntry) error {
	return f(ctx, entry)
}

// Close is a no-op for WriterFunc.
func (f WriterFunc) Close() error {
	return nil
}

// Ensure WriterFunc implements Writer interface.
var _ Writer = (WriterFunc)(nil)

// MultiWriter writes to multiple writers concurrently.
// Errors from individual writers are collected but don't stop writes to other writers.
//
// Thread-safe via mutex.
type MultiWriter struct {
	mu      sync.RWMutex
	writers []Writer
	closed  bool
}

// NewMultiWriter creates a new MultiWriter that writes to all provided writers.
//
// Usage:
//
//	stdout := logger.NewStdoutWriter("json")
//	file, _ := logger.NewFileWriter(config, "json")
//	mw := logger.NewMultiWriter(stdout, file)
//	defer mw.Close()
func NewMultiWriter(writers ...Writer) *MultiWriter {
	// Filter out nil writers
	filtered := make([]Writer, 0, len(writers))
	for _, w := range writers {
		if w != nil {
			filtered = append(filtered, w)
		}
	}

	return &MultiWriter{
		writers: filtered,
		closed:  false,
	}
}

// Write writes the log entry to all writers.
// If a writer fails, the error is logged to stderr but other writers continue.
// Returns error only if all writers fail.
func (m *MultiWriter) Write(ctx context.Context, entry LogEntry) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return fmt.Errorf("multiwriter is closed")
	}

	if len(m.writers) == 0 {
		return nil
	}

	var firstErr error
	successCount := 0

	for _, w := range m.writers {
		if err := w.Write(ctx, entry); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			// Log error to stderr but continue with other writers
			fmt.Fprintf(os.Stderr, "writer error: %v\n", err)
		} else {
			successCount++
		}
	}

	// Return error only if all writers failed
	if successCount == 0 && firstErr != nil {
		return fmt.Errorf("all writers failed: %w", firstErr)
	}

	return nil
}

// Close closes all writers and aggregates any errors.
// Safe to call multiple times. Subsequent calls return nil.
func (m *MultiWriter) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil
	}

	m.closed = true

	var firstErr error
	for _, w := range m.writers {
		if err := w.Close(); err != nil {
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	return firstErr
}

// AddWriter adds a writer to the MultiWriter.
// Safe to call concurrently with Write.
// Returns error if MultiWriter is closed.
func (m *MultiWriter) AddWriter(w Writer) error {
	if w == nil {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return fmt.Errorf("multiwriter is closed")
	}

	m.writers = append(m.writers, w)
	return nil
}

// RemoveWriter removes a writer from the MultiWriter.
// Returns true if writer was found and removed.
//
// LIMITATION: This method uses equality comparison (==), so it may not work
// correctly with WriterFunc or other non-comparable writer implementations.
// It works correctly with StdoutWriter, FileWriter, and SyslogWriter pointers.
func (m *MultiWriter) RemoveWriter(w Writer) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return false
	}

	for i, writer := range m.writers {
		if writer == w {
			// Remove by swapping with last element
			m.writers[i] = m.writers[len(m.writers)-1]
			m.writers = m.writers[:len(m.writers)-1]
			return true
		}
	}

	return false
}

// Writers returns a copy of the current writer slice.
// Safe for read-only access. Writers may be stale if Add/Remove called.
func (m *MultiWriter) Writers() []Writer {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Writer, len(m.writers))
	copy(result, m.writers)
	return result
}

// Len returns the number of writers in the MultiWriter.
func (m *MultiWriter) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.writers)
}

// Ensure MultiWriter implements Writer interface.
var _ Writer = (*MultiWriter)(nil)
