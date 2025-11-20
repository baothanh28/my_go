package rate

// Logger is the interface for logging rate limiter events
type Logger interface {
	// Debug logs a debug message
	Debug(msg string, keysAndValues ...interface{})

	// Info logs an info message
	Info(msg string, keysAndValues ...interface{})

	// Warn logs a warning message
	Warn(msg string, keysAndValues ...interface{})

	// Error logs an error message
	Error(msg string, keysAndValues ...interface{})
}

// NoOpLogger is a no-op logger implementation
type NoOpLogger struct{}

func (l *NoOpLogger) Debug(msg string, keysAndValues ...interface{}) {}
func (l *NoOpLogger) Info(msg string, keysAndValues ...interface{})  {}
func (l *NoOpLogger) Warn(msg string, keysAndValues ...interface{})  {}
func (l *NoOpLogger) Error(msg string, keysAndValues ...interface{}) {}
