package gortsplib

// LogLevel is a log level.
//
// Deprecated: Log() is deprecated.
type LogLevel int

// Log levels.
//
// Deprecated: Log() is deprecated.
const (
	LogLevelDebug LogLevel = iota + 1
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)
