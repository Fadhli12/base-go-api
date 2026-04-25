// Package logger provides a structured logging abstraction layer over log/slog.
package logger

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// LoggedMessage represents a single logged message captured by MockLogger
type LoggedMessage struct {
	Level   Level
	Message string
	Fields  []Field
	Context context.Context
}

// MockLogger is a test implementation of Logger that records all log calls
// for later inspection. Thread-safe for use in concurrent tests.
//
// Usage:
//
//	func TestHandler(t *testing.T) {
//	    mockLog := logger.NewMockLogger()
//	    handler := NewHandler(mockLog, ...)
//	    
//	    // Call handler
//	    handler.ProcessRequest(...)
//	    
//	    // Assert log calls
//	    assert.True(t, mockLog.AssertLogged(logger.LevelInfo, "request processed"))
//	    assert.True(t, mockLog.AssertField("user_id", "123"))
//	}
type MockLogger struct {
	mu        sync.RWMutex
	messages  []LoggedMessage
	msgPtr    *[]LoggedMessage // Shared pointer to message slice
	fields    []Field          // Base fields added via WithFields
	err       error            // Error added via WithError
}

// NewMockLogger creates a new MockLogger for testing
func NewMockLogger() *MockLogger {
	messages := make([]LoggedMessage, 0)
	return &MockLogger{
		messages:  messages,
		msgPtr:    &messages, // Shared reference
		fields:    make([]Field, 0),
	}
}

// Debug records a DEBUG level log message
func (m *MockLogger) Debug(ctx context.Context, msg string, fields ...Field) {
	m.record(LevelDebug, msg, ctx, fields...)
}

// Info records an INFO level log message
func (m *MockLogger) Info(ctx context.Context, msg string, fields ...Field) {
	m.record(LevelInfo, msg, ctx, fields...)
}

// Warn records a WARN level log message
func (m *MockLogger) Warn(ctx context.Context, msg string, fields ...Field) {
	m.record(LevelWarn, msg, ctx, fields...)
}

// Error records an ERROR level log message
func (m *MockLogger) Error(ctx context.Context, msg string, fields ...Field) {
	m.record(LevelError, msg, ctx, fields...)
}

// record adds a message to the internal log with proper synchronization
func (m *MockLogger) record(level Level, msg string, ctx context.Context, fields ...Field) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Merge base fields with provided fields
	allFields := make([]Field, 0, len(m.fields)+len(fields)+1) // +1 for potential error
	allFields = append(allFields, m.fields...)
	allFields = append(allFields, fields...)

	// Add error field if present
	if m.err != nil {
		allFields = append(allFields, Field{Key: "error", Value: m.err.Error()})
	}

	*m.msgPtr = append(*m.msgPtr, LoggedMessage{
		Level:   level,
		Message: msg,
		Fields:  allFields,
		Context: ctx,
	})
}

// WithFields returns a new MockLogger with additional base fields
// The new logger shares the same message slice for assertions
func (m *MockLogger) WithFields(fields ...Field) Logger {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Copy base fields and append new ones
	newFields := make([]Field, len(m.fields))
	copy(newFields, m.fields)
	newFields = append(newFields, fields...)

	return &MockLogger{
		messages:  m.messages,
		msgPtr:    m.msgPtr, // Shared reference
		fields:    newFields,
		err:       m.err,
	}
}

// WithError returns a new MockLogger with an error attached
func (m *MockLogger) WithError(err error) Logger {
	m.mu.Lock()
	defer m.mu.Unlock()

	newFields := make([]Field, len(m.fields))
	copy(newFields, m.fields)

	return &MockLogger{
		messages:  m.messages,
		msgPtr:    m.msgPtr, // Shared reference
		fields:    newFields,
		err:       err,
	}
}

// Field factory methods - return Field values

func (m *MockLogger) String(key, value string) Field {
	return Field{Key: key, Value: value}
}

func (m *MockLogger) Int(key string, value int) Field {
	return Field{Key: key, Value: value}
}

func (m *MockLogger) Int64(key string, value int64) Field {
	return Field{Key: key, Value: value}
}

func (m *MockLogger) Float64(key string, value float64) Field {
	return Field{Key: key, Value: value}
}

func (m *MockLogger) Bool(key string, value bool) Field {
	return Field{Key: key, Value: value}
}

func (m *MockLogger) Duration(key string, value time.Duration) Field {
	return Field{Key: key, Value: value}
}

func (m *MockLogger) Time(key string, value time.Time) Field {
	return Field{Key: key, Value: value}
}

func (m *MockLogger) Any(key string, value interface{}) Field {
	return Field{Key: key, Value: value}
}

// AssertLogged returns true if a message with the given level and containing
// the given substring was logged
//
// Usage:
//
//	assert.True(t, mockLog.AssertLogged(logger.LevelInfo, "user created"))
func (m *MockLogger) AssertLogged(level Level, msgSubstring string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, msg := range *m.msgPtr {
		if msg.Level == level && containsSubstring(msg.Message, msgSubstring) {
			return true
		}
	}
	return false
}

// AssertLoggedExactly returns true if a message with the exact level and message was logged
func (m *MockLogger) AssertLoggedExactly(level Level, message string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, msg := range *m.msgPtr {
		if msg.Level == level && msg.Message == message {
			return true
		}
	}
	return false
}

// AssertField returns true if a field with the given key and value exists
// in any logged message
//
// Usage:
//
//	assert.True(t, mockLog.AssertField("user_id", "123"))
func (m *MockLogger) AssertField(key string, value interface{}) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, msg := range *m.msgPtr {
		for _, field := range msg.Fields {
			if field.Key == key && fieldValuesEqual(field.Value, value) {
				return true
			}
		}
	}
	return false
}

// AssertFieldInMessage returns true if a field with the given key and value
// exists in a message logged at the specified level containing the message substring
func (m *MockLogger) AssertFieldInMessage(level Level, msgSubstring string, key string, value interface{}) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, msg := range *m.msgPtr {
		if msg.Level == level && containsSubstring(msg.Message, msgSubstring) {
			for _, field := range msg.Fields {
				if field.Key == key && fieldValuesEqual(field.Value, value) {
					return true
				}
			}
		}
	}
	return false
}

// GetMessages returns a copy of all logged messages
func (m *MockLogger) GetMessages() []LoggedMessage {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]LoggedMessage, len(*m.msgPtr))
	for i, msg := range *m.msgPtr {
		// Copy the message
		result[i] = LoggedMessage{
			Level:   msg.Level,
			Message: msg.Message,
			Context: msg.Context,
			Fields:  make([]Field, len(msg.Fields)),
		}
		copy(result[i].Fields, msg.Fields)
	}
	return result
}

// GetMessagesCount returns the number of logged messages
func (m *MockLogger) GetMessagesCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(*m.msgPtr)
}

// GetMessagesAtLevel returns all messages logged at the specified level
func (m *MockLogger) GetMessagesAtLevel(level Level) []LoggedMessage {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []LoggedMessage
	for _, msg := range *m.msgPtr {
		if msg.Level == level {
			result = append(result, msg)
		}
	}
	return result
}

// Reset clears all recorded messages
func (m *MockLogger) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	*m.msgPtr = (*m.msgPtr)[:0]
}

// FindMessage returns the first message matching the level and substring, or nil
func (m *MockLogger) FindMessage(level Level, msgSubstring string) *LoggedMessage {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Use the shared message pointer to ensure we see all messages
	// even if this logger was created via WithFields or WithError
	messages := *m.msgPtr
	for i := range messages {
		if messages[i].Level == level && containsSubstring(messages[i].Message, msgSubstring) {
			// Return a copy, not a pointer to the slice element
			msgCopy := messages[i]
			fieldsCopy := make([]Field, len(msgCopy.Fields))
			copy(fieldsCopy, msgCopy.Fields)
			msgCopy.Fields = fieldsCopy
			return &msgCopy
		}
	}
	return nil
}

// HasError returns true if any logged message contains an error field
func (m *MockLogger) HasError() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	messages := *m.msgPtr
	for _, msg := range messages {
		for _, field := range msg.Fields {
			if field.Key == "error" && field.Value != nil {
				return true
			}
		}
	}
	return false
}

// Helper functions

func containsSubstring(s, substr string) bool {
	if substr == "" {
		return true
	}
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstringHelper(s, substr))
}

func containsSubstringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func fieldValuesEqual(a, b interface{}) bool {
	// Handle nil cases
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Convert to comparable types
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}
