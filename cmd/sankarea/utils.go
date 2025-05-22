package main

import (
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

// LoadEnv loads environment variables from .env file
func LoadEnv() {
	godotenv.Load()
}

// FileMustExist checks if a file exists and is readable
// If not, it creates a default version of the file
func FileMustExist(path string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Create directory if needed
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			Logger().Printf("Failed to create directory %s: %v", dir, err)
			os.Exit(1)
		}

		// Create empty file
		file, err := os.Create(path)
		if err != nil {
			Logger().Printf("Failed to create file %s: %v", path, err)
			os.Exit(1)
		}
		file.Close()

		Logger().Printf("Created empty file: %s", path)
	}
}

// EnsureDataDir ensures the data directory exists
func EnsureDataDir() {
	if err := os.MkdirAll("data", 0755); err != nil {
		Logger().Printf("Failed to create data directory: %v", err)
		os.Exit(1)
	}
}
