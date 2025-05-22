package main

import (
    "bufio"
    "log"
    "os"
    "strings"
)

// LoadEnv reads .env and sets environment variables if they are not already set
func LoadEnv() {
    if _, err := os.Stat(".env"); err == nil {
        file, err := os.Open(".env")
        if err == nil {
            defer file.Close()
            scanner := bufio.NewScanner(file)
            for scanner.Scan() {
                line := scanner.Text()
                if strings.HasPrefix(line, "#") || len(strings.TrimSpace(line)) == 0 {
                    continue
                }
                parts := strings.SplitN(line, "=", 2)
                if len(parts) != 2 {
                    continue
                }
                key := strings.TrimSpace(parts[0])
                // strip surrounding quotes (both " and ')
                val := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
                if os.Getenv(key) == "" {
                    os.Setenv(key, val)
                }
            }
        }
    }
}

// GetEnvOrFail retrieves an environment variable or fatals if missing
func GetEnvOrFail(key string) string {
    v := os.Getenv(key)
    if v == "" {
        log.Fatalf("Missing required environment variable: %s", key)
    }
    return v
}

// GetEnvOrDefault retrieves an environment variable or returns default
func GetEnvOrDefault(key, defaultValue string) string {
    v := os.Getenv(key)
    if v == "" {
        return defaultValue
    }
    return v
}

// FileMustExist fatals if a required file is missing
func FileMustExist(path string) {
    if _, err := os.Stat(path); os.IsNotExist(err) {
        log.Fatalf("Required file not found: %s", path)
    }
}

// EnsureDataDir creates the data directory if it does not exist
func EnsureDataDir() {
    if _, err := os.Stat("data"); os.IsNotExist(err) {
        os.Mkdir("data", 0755)
    }
}
