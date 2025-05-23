// cmd/sankarea/logging.go
package main

import (
    "fmt"
    "io"
    "log"
    "os"
    "path/filepath"
    "sync"
    "time"
)

const (
    logFileFormat     = "2006-01-02"
    logTimeFormat     = "2006-01-02 15:04:05"
    maxLogFiles       = 30
    logFileSizeLimit  = 10 * 1024 * 1024 // 10MB
)

var (
    logger     *log.Logger
    logFile    *os.File
    logMutex   sync.RWMutex
    auditFile  *os.File
    auditMutex sync.RWMutex
)

// LogLevel represents the severity of a log message
type LogLevel int

const (
    LogDebug LogLevel = iota
    LogInfo
    LogWarn
    LogError
)

// LogEntry represents a structured log entry
type LogEntry struct {
    Timestamp time.Time `json:"timestamp"`
    Level     LogLevel  `json:"level"`
    Message   string    `json:"message"`
    Component string    `json:"component"`
    Error     error    `json:"error,omitempty"`
}

// InitializeLogging sets up the logging system
func InitializeLogging(level string) error {
    logMutex.Lock()
    defer logMutex.Unlock()

    // Create logs directory if it doesn't exist
    if err := os.MkdirAll("data/logs", 0755); err != nil {
        return fmt.Errorf("failed to create logs directory: %v", err)
    }

    // Set up main log file
    if err := setupLogFile(); err != nil {
        return fmt.Errorf("failed to set up log file: %v", err)
    }

    // Set up audit log file
    if err := setupAuditLog(); err != nil {
        return fmt.Errorf("failed to set up audit log: %v", err)
    }

    // Configure logger
    writers := []io.Writer{os.Stdout}
    if logFile != nil {
        writers = append(writers, logFile)
    }

    logger = log.New(io.MultiWriter(writers...), "", 0)

    // Clean up old log files
    go cleanupOldLogs()

    logger.Printf("Logging initialized at level: %s", level)
    return nil
}

// setupLogFile creates and sets up the main log file
func setupLogFile() error {
    filename := fmt.Sprintf("sankarea-%s.log", time.Now().Format(logFileFormat))
    path := filepath.Join("data/logs", filename)

    file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
    if err != nil {
        return fmt.Errorf("failed to open log file: %v", err)
    }

    if logFile != nil {
        logFile.Close()
    }
    logFile = file

    return nil
}

// setupAuditLog creates and sets up the audit log file
func setupAuditLog() error {
    filename := fmt.Sprintf("audit-%s.log", time.Now().Format(logFileFormat))
    path := filepath.Join("data/logs", filename)

    file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
    if err != nil {
        return fmt.Errorf("failed to open audit log file: %v", err)
    }

    if auditFile != nil {
        auditFile.Close()
    }
    auditFile = file

    return nil
}

// Logger returns the global logger instance
func Logger() *log.Logger {
    logMutex.RLock()
    defer logMutex.RUnlock()
    return logger
}

// AuditLog logs an audit event
func AuditLog(message string) {
    auditMutex.Lock()
    defer auditMutex.Unlock()

    timestamp := time.Now().Format(logTimeFormat)
    entry := fmt.Sprintf("[%s] %s\n", timestamp, message)

    if auditFile != nil {
        auditFile.WriteString(entry)
    }
}

// LogError logs an error with context
func LogError(component string, err error, message string) {
    entry := LogEntry{
        Timestamp: time.Now(),
        Level:     LogError,
        Component: component,
        Message:   message,
        Error:     err,
    }

    logMutex.Lock()
    defer logMutex.Unlock()

    logger.Printf("[ERROR] [%s] %s: %v", component, message, err)

    // Store error in error buffer
    if errorBuffer != nil {
        errorBuffer.Add(&ErrorEvent{
            Time:      entry.Timestamp,
            Component: component,
            Message:   fmt.Sprintf("%s: %v", message, err),
            Severity:  "error",
        })
    }
}

// cleanupOldLogs removes log files older than the retention period
func cleanupOldLogs() {
    logDir := "data/logs"
    files, err := os.ReadDir(logDir)
    if err != nil {
        logger.Printf("Failed to read logs directory: %v", err)
        return
    }

    // Get list of log files with their modification times
    type fileInfo struct {
        name    string
        modTime time.Time
    }
    var logFiles []fileInfo

    for _, file := range files {
        if !file.IsDir() {
            info, err := file.Info()
            if err != nil {
                continue
            }
            logFiles = append(logFiles, fileInfo{
                name:    file.Name(),
                modTime: info.ModTime(),
            })
        }
    }

    // Sort files by modification time (oldest first)
    // Using bubble sort for simplicity, can be optimized if needed
    for i := 0; i < len(logFiles)-1; i++ {
        for j := 0; j < len(logFiles)-i-1; j++ {
            if logFiles[j].modTime.After(logFiles[j+1].modTime) {
                logFiles[j], logFiles[j+1] = logFiles[j+1], logFiles[j]
            }
        }
    }

    // Remove old files while keeping the most recent ones
    if len(logFiles) > maxLogFiles {
        for _, file := range logFiles[:len(logFiles)-maxLogFiles] {
            path := filepath.Join(logDir, file.name)
            if err := os.Remove(path); err != nil {
                logger.Printf("Failed to remove old log file %s: %v", path, err)
            }
        }
    }
}

// rotateLogFiles checks and rotates log files if they exceed size limit
func rotateLogFiles() error {
    logMutex.Lock()
    defer logMutex.Unlock()

    if logFile == nil {
        return nil
    }

    info, err := logFile.Stat()
    if err != nil {
        return fmt.Errorf("failed to stat log file: %v", err)
    }

    if info.Size() < logFileSizeLimit {
        return nil
    }

    // Close current log file
    logFile.Close()

    // Set up new log file
    return setupLogFile()
}

// Shutdown closes all log files
func ShutdownLogging() error {
    logMutex.Lock()
    defer logMutex.Unlock()

    if logFile != nil {
        if err := logFile.Close(); err != nil {
            return fmt.Errorf("failed to close log file: %v", err)
        }
    }

    if auditFile != nil {
        if err := auditFile.Close(); err != nil {
            return fmt.Errorf("failed to close audit file: %v", err)
        }
    }

    return nil
}
