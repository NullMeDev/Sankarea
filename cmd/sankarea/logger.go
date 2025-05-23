// cmd/sankarea/logger.go
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

// LogLevel represents the severity of a log message
type LogLevel int

const (
    LogDebug LogLevel = iota
    LogInfo
    LogWarning
    LogError
)

var logLevelStrings = map[LogLevel]string{
    LogDebug:   "DEBUG",
    LogInfo:    "INFO",
    LogWarning: "WARN",
    LogError:   "ERROR",
}

// Logger handles application logging
type Logger struct {
    logger     *log.Logger
    file       *os.File
    level      LogLevel
    filename   string
    maxSize    int64
    mutex      sync.Mutex
    startTime  time.Time
}

var (
    instance *Logger
    once     sync.Once
)

// InitLogger initializes the global logger instance
func InitLogger() error {
    var err error
    once.Do(func() {
        instance, err = newLogger(cfg.LogPath, cfg.LogLevel)
    })
    return err
}

// Logger returns the global logger instance
func Logger() *Logger {
    if instance == nil {
        panic("Logger not initialized")
    }
    return instance
}

// newLogger creates a new logger instance
func newLogger(logPath string, level LogLevel) (*Logger, error) {
    // Create log directory if it doesn't exist
    if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
        return nil, fmt.Errorf("failed to create log directory: %v", err)
    }

    // Open log file
    file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
    if err != nil {
        return nil, fmt.Errorf("failed to open log file: %v", err)
    }

    // Create multi-writer for both file and stdout
    multiWriter := io.MultiWriter(file, os.Stdout)

    l := &Logger{
        logger:    log.New(multiWriter, "", log.LstdFlags),
        file:      file,
        level:     level,
        filename:  logPath,
        maxSize:   50 * 1024 * 1024, // 50MB
        startTime: time.Now(),
    }

    l.Info("Logger initialized")
    return l, nil
}

// log formats and writes a log message
func (l *Logger) log(level LogLevel, format string, args ...interface{}) {
    if level < l.level {
        return
    }

    l.mutex.Lock()
    defer l.mutex.Unlock()

    // Check if rotation is needed
    if err := l.rotateIfNeeded(); err != nil {
        fmt.Fprintf(os.Stderr, "Failed to rotate log file: %v\n", err)
    }

    // Format message with level prefix
    msg := fmt.Sprintf("[%s] %s", logLevelStrings[level], fmt.Sprintf(format, args...))
    l.logger.Print(msg)
}

// Debug logs a debug message
func (l *Logger) Debug(format string, args ...interface{}) {
    l.log(LogDebug, format, args...)
}

// Info logs an info message
func (l *Logger) Info(format string, args ...interface{}) {
    l.log(LogInfo, format, args...)
}

// Warning logs a warning message
func (l *Logger) Warning(format string, args ...interface{}) {
    l.log(LogWarning, format, args...)
}

// Error logs an error message
func (l *Logger) Error(format string, args ...interface{}) {
    l.log(LogError, format, args...)
}

// rotateIfNeeded checks if log rotation is needed and performs it
func (l *Logger) rotateIfNeeded() error {
    info, err := l.file.Stat()
    if err != nil {
        return fmt.Errorf("failed to stat log file: %v", err)
    }

    if info.Size() < l.maxSize {
        return nil
    }

    // Close current file
    if err := l.file.Close(); err != nil {
        return fmt.Errorf("failed to close log file: %v", err)
    }

    // Generate new filename with timestamp
    timestamp := time.Now().Format("20060102-150405")
    rotatedPath := fmt.Sprintf("%s.%s", l.filename, timestamp)

    // Rename current file
    if err := os.Rename(l.filename, rotatedPath); err != nil {
        return fmt.Errorf("failed to rename log file: %v", err)
    }

    // Open new file
    file, err := os.OpenFile(l.filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
    if err != nil {
        return fmt.Errorf("failed to open new log file: %v", err)
    }

    // Update logger
    multiWriter := io.MultiWriter(file, os.Stdout)
    l.logger.SetOutput(multiWriter)
    l.file = file

    l.Info("Log file rotated to %s", rotatedPath)
    return nil
}

// cleanOldLogs removes log files older than the retention period
func (l *Logger) cleanOldLogs() error {
    dir := filepath.Dir(l.filename)
    pattern := filepath.Base(l.filename) + ".*"
    
    files, err := filepath.Glob(filepath.Join(dir, pattern))
    if err != nil {
        return fmt.Errorf("failed to list log files: %v", err)
    }

    retention := 7 * 24 * time.Hour // 7 days
    now := time.Now()

    for _, file := range files {
        info, err := os.Stat(file)
        if err != nil {
            l.Warning("Failed to stat log file %s: %v", file, err)
            continue
        }

        if now.Sub(info.ModTime()) > retention {
            if err := os.Remove(file); err != nil {
                l.Warning("Failed to remove old log file %s: %v", file, err)
                continue
            }
            l.Info("Removed old log file: %s", file)
        }
    }

    return nil
}

// Close closes the logger and underlying file
func (l *Logger) Close() error {
    l.mutex.Lock()
    defer l.mutex.Unlock()

    if err := l.file.Close(); err != nil {
        return fmt.Errorf("failed to close log file: %v", err)
    }

    return nil
}

// GetStats returns logging statistics
func (l *Logger) GetStats() map[string]interface{} {
    l.mutex.Lock()
    defer l.mutex.Unlock()

    info, err := l.file.Stat()
    size := int64(0)
    if err == nil {
        size = info.Size()
    }

    return map[string]interface{}{
        "uptime":      time.Since(l.startTime).String(),
        "level":       logLevelStrings[l.level],
        "currentSize": size,
        "maxSize":     l.maxSize,
    }
}

// SetLevel changes the logging level
func (l *Logger) SetLevel(level LogLevel) {
    l.mutex.Lock()
    defer l.mutex.Unlock()
    l.level = level
    l.Info("Log level changed to %s", logLevelStrings[level])
}
