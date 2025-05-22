package main

import (
    "fmt"
    "io"
    "log"
    "os"
    "path/filepath"
    "runtime"
    "time"
)

var (
    logger  *log.Logger
    logFile *os.File
)

// SetupLogging initializes console + file logger
func SetupLogging() error {
    if err := os.MkdirAll("logs", 0755); err != nil {
        return err
    }
    name := fmt.Sprintf("logs/sankarea_%s.log", time.Now().Format("2006-01-02"))
    var err error
    logFile, err = os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
    if err != nil {
        return err
    }
    mw := io.MultiWriter(os.Stdout, logFile)
    logger = log.New(mw, "", log.Ldate|log.Ltime|log.Lshortfile)
    log.SetOutput(mw)
    log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
    return nil
}

// Logf logs a formatted message with file:line prefix
func Logf(format string, args ...interface{}) {
    _, file, line, _ := runtime.Caller(1)
    logger.Printf("%s:%d: %s", filepath.Base(file), line, fmt.Sprintf(format, args...))
}

// Logger returns the underlying *log.Logger
func Logger() *log.Logger {
    return logger
}
