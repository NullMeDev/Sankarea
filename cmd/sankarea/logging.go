package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

// ErrorRecord represents a single error record
type ErrorRecord struct {
	Time      time.Time
	Message   string
	Error     string
	Component string
	Severity  int
}

// ErrorSystem manages error logging and tracking
type ErrorSystem struct {
	errors      []ErrorRecord
	errorsMutex sync.Mutex
	maxErrors   int
}

// NewErrorSystem creates a new error system
func NewErrorSystem(maxErrors int) *ErrorSystem {
	if maxErrors <= 0 {
		maxErrors = 100
	}
	return &ErrorSystem{
		errors:    make([]ErrorRecord, 0, maxErrors),
		maxErrors: maxErrors,
	}
}

// HandleError records an error and takes appropriate action based on severity
func (es *ErrorSystem) HandleError(message string, err error, component string, severity int) {
	// Always log the error
	Logger().Printf("[%s] %s: %v", severityString(severity), message, err)

	// Record error
	es.errorsMutex.Lock()
	defer es.errorsMutex.Unlock()

	// Add the error to our records
	record := ErrorRecord{
		Time:      time.Now(),
		Message:   message,
		Error:     err.Error(),
		Component: component,
		Severity:  severity,
	}

	// If we've reached capacity, remove the oldest entry
	if len(es.errors) >= es.maxErrors {
		es.errors = es.errors[1:]
	}

	// Add the new error
	es.errors = append(es.errors, record)

	// Increment the global error counter
	IncrementErrorCount()

	// If this is a fatal error, exit
	if severity == ErrorSeverityFatal {
		Logger().Printf("FATAL ERROR: %s - %v", message, err)
		os.Exit(1)
	}
}

// GetErrors returns the recorded errors
func (es *ErrorSystem) GetErrors() []ErrorRecord {
	es.errorsMutex.Lock()
	defer es.errorsMutex.Unlock()

	// Return a copy to avoid race conditions
	result := make([]ErrorRecord, len(es.errors))
	copy(result, es.errors)
	return result
}

// severityString converts severity level to string representation
func severityString(severity int) string {
	switch severity {
	case ErrorSeverityLow:
		return "LOW"
	case ErrorSeverityMedium:
		return "MEDIUM"
	case ErrorSeverityHigh:
		return "HIGH"
	case ErrorSeverityFatal:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// WriteErrorLog writes all current errors to a log file
func (es *ErrorSystem) WriteErrorLog() error {
	// Create error logs directory if it doesn't exist
	logDir := "logs/errors"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}

	// Create log file with timestamp
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	logFile := filepath.Join(logDir, fmt.Sprintf("errors_%s.log", timestamp))

	// Open file for writing
	f, err := os.Create(logFile)
	if err != nil {
		return err
	}
	defer f.Close()

	// Write header
	f.WriteString("# Sankarea Error Log\n")
	f.WriteString(fmt.Sprintf("# Generated: %s\n", time.Now().Format(time.RFC3339)))
	f.WriteString("# Format: [TIME] [SEVERITY] [COMPONENT] MESSAGE: ERROR\n\n")

	// Write all errors
	errors := es.GetErrors()
	for _, e := range errors {
		f.WriteString(fmt.Sprintf("[%s] [%s] [%s] %s: %s\n",
			e.Time.Format(time.RFC3339),
			severityString(e.Severity),
			e.Component,
			e.Message,
			e.Error))
	}

	return nil
}

// AuditLog adds an entry to the audit log for admin actions
func AuditLog(s *discordgo.Session, action, userID, details string) {
	// Create audit logs directory if it doesn't exist
	logDir := "logs/audit"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		Logger().Printf("Failed to create audit log directory: %v", err)
		return
	}

	// Get current log file (one per day)
	logFile := filepath.Join(logDir, fmt.Sprintf("audit_%s.log", 
		time.Now().Format("2006-01-02")))

	// Open file for appending
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		Logger().Printf("Failed to open audit log file: %v", err)
		return
	}
	defer f.Close()

	// Write entry
	timestamp := time.Now().Format("15:04:05")
	entry := fmt.Sprintf("[%s] %s by %s: %s\n", timestamp, action, userID, details)
	if _, err := f.WriteString(entry); err != nil {
		Logger().Printf("Failed to write to audit log: %v", err)
	}
}
