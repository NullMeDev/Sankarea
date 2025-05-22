package main

import (
	"log"
	"os"
	"path/filepath"
	"time"
)

var logger *log.Logger

// SetupLogging configures logging to both console and file
func SetupLogging() error {
	// Create logs directory if it doesn't exist
	err := os.MkdirAll("logs", 0755)
	if err != nil {
		return err
	}

	// Create log file with timestamp
	timestamp := time.Now().Format("2006-01-02")
	logFile, err := os.OpenFile(
		filepath.Join("logs", "sankarea-"+timestamp+".log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0644,
	)
	if err != nil {
		return err
	}

	// Configure logger to write to both file and stderr
	logger = log.New(os.Stderr, "", log.LstdFlags)
	log.SetOutput(logFile)

	return nil
}

// Logger returns the configured logger
func Logger() *log.Logger {
	if logger == nil {
		// Fallback to standard logger if not configured
		return log.Default()
	}
	return logger
}
