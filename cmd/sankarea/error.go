// cmd/sankarea/errors.go
package main

import (
    "fmt"
    "runtime"
    "strings"
    "time"
)

// ErrorType represents different categories of errors
type ErrorType string

const (
    // Error types
    ErrorTypeDiscord   ErrorType = "discord"
    ErrorTypeDatabase  ErrorType = "database"
    ErrorTypeNews      ErrorType = "news"
    ErrorTypeConfig    ErrorType = "config"
    ErrorTypeScheduler ErrorType = "scheduler"
    ErrorTypeAPI       ErrorType = "api"
    ErrorTypeInternal  ErrorType = "internal"
)

// ErrorEvent represents a recorded error event
type ErrorEvent struct {
    Type      ErrorType `json:"type"`
    Code      string    `json:"code"`
    Message   string    `json:"message"`
    Component string    `json:"component"`
    Stack     string    `json:"stack,omitempty"`
    Time      time.Time `json:"time"`
}

// SankareaError is the custom error type for the application
type SankareaError struct {
    Type      ErrorType `json:"type"`
    Code      string    `json:"code"`
    Message   string    `json:"message"`
    Component string    `json:"component"`
    Inner     error    `json:"inner,omitempty"`
}

func (e *SankareaError) Error() string {
    if e.Inner != nil {
        return fmt.Sprintf("[%s-%s] %s: %v", e.Type, e.Code, e.Message, e.Inner)
    }
    return fmt.Sprintf("[%s-%s] %s", e.Type, e.Code, e.Message)
}

// NewError creates a new SankareaError
func NewError(errType ErrorType, code string, message string, inner error) *SankareaError {
    return &SankareaError{
        Type:    errType,
        Code:    code,
        Message: message,
        Inner:   inner,
    }
}

// Common error constructors
func NewDiscordError(code string, message string, inner error) *SankareaError {
    return NewError(ErrorTypeDiscord, code, message, inner)
}

func NewDatabaseError(code string, message string, inner error) *SankareaError {
    return NewError(ErrorTypeDatabase, code, message, inner)
}

func NewNewsError(code string, message string, inner error) *SankareaError {
    return NewError(ErrorTypeNews, code, message, inner)
}

func NewConfigError(code string, message string, inner error) *SankareaError {
    return NewError(ErrorTypeConfig, code, message, inner)
}

func NewSchedulerError(code string, message string, inner error) *SankareaError {
    return NewError(ErrorTypeScheduler, code, message, inner)
}

// Error codes
const (
    // Discord error codes
    ErrDiscordConnection = "DISCORD_001"
    ErrDiscordPermission = "DISCORD_002"
    ErrDiscordRateLimit  = "DISCORD_003"
    
    // Database error codes
    ErrDatabaseConnection = "DB_001"
    ErrDatabaseQuery     = "DB_002"
    ErrDatabaseMigration = "DB_003"
    
    // News error codes
    ErrNewsFetch        = "NEWS_001"
    ErrNewsParser       = "NEWS_002"
    ErrNewsRateLimit    = "NEWS_003"
    
    // Config error codes
    ErrConfigLoad       = "CONFIG_001"
    ErrConfigValidation = "CONFIG_002"
    
    // Scheduler error codes
    ErrSchedulerTask    = "SCHED_001"
    ErrSchedulerTimeout = "SCHED_002"
)

// ErrorHandler handles and records errors
type ErrorHandler struct {
    buffer *ErrorBuffer
}

// NewErrorHandler creates a new error handler
func NewErrorHandler(bufferSize int) *ErrorHandler {
    return &ErrorHandler{
        buffer: NewErrorBuffer(bufferSize),
    }
}

// Handle processes and records an error
func (h *ErrorHandler) Handle(err error, component string) {
    if err == nil {
        return
    }

    // Create error event
    event := &ErrorEvent{
        Time:      time.Now(),
        Component: component,
        Stack:     getStackTrace(),
    }

    // Extract error details
    if se, ok := err.(*SankareaError); ok {
        event.Type = se.Type
        event.Code = se.Code
        event.Message = se.Message
    } else {
        event.Type = ErrorTypeInternal
        event.Code = "INTERNAL_001"
        event.Message = err.Error()
    }

    // Add to buffer
    h.buffer.Add(event)

    // Log the error
    Logger().Printf("[ERROR] %s: %v", component, err)

    // Update error metrics
    IncrementCounter("error")

    // Update component status if applicable
    if component != "" {
        UpdateComponentStatus(component, "error", err)
    }
}

// getStackTrace returns the current stack trace
func getStackTrace() string {
    const depth = 32
    var pcs [depth]uintptr
    n := runtime.Callers(3, pcs[:])
    frames := runtime.CallersFrames(pcs[:n])

    var trace []string
    for {
        frame, more := frames.Next()
        // Skip runtime and standard library frames
        if !strings.Contains(frame.File, "runtime/") {
            trace = append(trace, fmt.Sprintf("%s:%d %s", frame.File, frame.Line, frame.Function))
        }
        if !more {
            break
        }
    }

    return strings.Join(trace, "\n")
}

// IsTransient determines if an error is likely temporary
func IsTransient(err error) bool {
    if se, ok := err.(*SankareaError); ok {
        switch se.Code {
        case ErrDiscordRateLimit,
             ErrNewsRateLimit,
             ErrDatabaseConnection,
             ErrSchedulerTimeout:
            return true
        }
    }
    return false
}

// GetErrorBuffer returns the error buffer
func (h *ErrorHandler) GetErrorBuffer() *ErrorBuffer {
    return h.buffer
}

// GetRecentErrors returns recent error events
func (h *ErrorHandler) GetRecentErrors(count int) []*ErrorEvent {
    return h.buffer.GetRecent(count)
}
