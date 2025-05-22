package main

import (
    "bufio"
    "log"
    "os"
    "strings"
)

// LoadEnv reads a .env file and sets variables if not already present
func LoadEnv() {
    if _, err := os.Stat(".env"); err == nil {
        file, err := os.Open(".env")
        if err != nil {
            log.Printf("Error opening .env: %v", err)
            return
        }
        defer file.Close()
        scanner := bufio.NewScanner(file)
        for scanner.Scan() {
            line := strings.TrimSpace(scanner.Text())
            if line == "" || strings.HasPrefix(line, "#") {
                continue
            }
            parts := strings.SplitN(line, "=", 2)
            if len(parts) != 2 {
                continue
            }
            key := strings.TrimSpace(parts[0])
            val := strings.TrimSpace(parts[1])
            if os.Getenv(key) == "" {
                os.Setenv(key, val)
            }
        }
        if err := scanner.Err(); err != nil {
            log.Printf("Error reading .env: %v", err)
        }
    }
}

// FileMustExist fatals if a required file is missing
func FileMustExist(path string) {
    if _, err := os.Stat(path); os.IsNotExist(err) {
        log.Fatalf("ERROR: Required file not found: %s", path)
    }
}

// EnsureDataDir creates the data directory if it doesn't exist
func EnsureDataDir() {
    if _, err := os.Stat("data"); os.IsNotExist(err) {
        if err := os.Mkdir("data", 0755); err != nil {
            log.Fatalf("ERROR: Could not create data dir: %v", err)
        }
    }
}
