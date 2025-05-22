package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// ConfigManager handles configuration management with auto-reload capabilities
type ConfigManager struct {
	configPath     string
	reloadInterval time.Duration
	lastModified   time.Time
	watcher        *fsnotify.Watcher
	mutex          sync.RWMutex
	onReload       func(*Config)
}

// NewConfigManager creates a new configuration manager
func NewConfigManager(configPath string, reloadInterval time.Duration) (*ConfigManager, error) {
	// Create watcher for file changes
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %v", err)
	}

	// Add config directory to watch
	configDir := filepath.Dir(configPath)
	if err := watcher.Add(configDir); err != nil {
		watcher.Close()
		return nil, fmt.Errorf("failed to watch directory %s: %v", configDir, err)
	}

	// Get initial file info
	fileInfo, err := os.Stat(configPath)
	var modTime time.Time
	if err == nil {
		modTime = fileInfo.ModTime()
	}

	manager := &ConfigManager{
		configPath:     configPath,
		reloadInterval: reloadInterval,
		lastModified:   modTime,
		watcher:        watcher,
		mutex:          sync.RWMutex{},
	}

	return manager, nil
}

// SetReloadHandler sets the callback function for config reloads
func (cm *ConfigManager) SetReloadHandler(handler func(*Config)) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	cm.onReload = handler
}

// StartWatching starts watching for configuration changes
func (cm *ConfigManager) StartWatching() {
	go cm.watchForChanges()
	go cm.periodicCheck()
}

// Stop stops the configuration watcher
func (cm *ConfigManager) Stop() {
	if cm.watcher != nil {
		cm.watcher.Close()
	}
}

// watchForChanges watches for file system events on the config file
func (cm *ConfigManager) watchForChanges() {
	for {
		select {
		case event, ok := <-cm.watcher.Events:
			if !ok {
				return
			}

			// Check if this is our config file
			if event.Name == cm.configPath && (event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create) {
				cm.checkAndReload()
			}

		case err, ok := <-cm.watcher.Errors:
			if !ok {
				return
			}
			Logger().Printf("Error watching config file: %v", err)
		}
	}
}

// periodicCheck periodically checks if the config file has changed
func (cm *ConfigManager) periodicCheck() {
	ticker := time.NewTicker(cm.reloadInterval)
	defer ticker.Stop()

	for {
		<-ticker.C
		cm.checkAndReload()
	}
}

// checkAndReload checks if the config file has changed and reloads it if necessary
func (cm *ConfigManager) checkAndReload() {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	fileInfo, err := os.Stat(cm.configPath)
	if err != nil {
		Logger().Printf("Error checking config file: %v", err)
		return
	}

	// Check if file has been modified since last load
	if fileInfo.ModTime().After(cm.lastModified) {
		Logger().Printf("Config file changed, reloading...")

		// Load new config
		newConfig, err := LoadConfig()
		if err != nil {
			Logger().Printf("Error reloading config: %v", err)
			return
		}

		// Update last modified time
		cm.lastModified = fileInfo.ModTime()

		// Call reload handler if set
		if cm.onReload != nil {
			cm.onReload(newConfig)
		}

		// Update global config
		cfg = newConfig

		Logger().Printf("Config successfully reloaded")
	}
}

// BackupConfig creates a timestamped backup of the current configuration
func BackupConfig() (string, error) {
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		return "", fmt.Errorf("config file does not exist")
	}

	// Create backups directory
	backupDir := filepath.Join("config", "backups")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create backup directory: %v", err)
	}

	// Generate backup filename with timestamp
	timestamp := time.Now().Format("20060102-150405")
	backupPath := filepath.Join(backupDir, fmt.Sprintf("config-%s.json", timestamp))

	// Copy the file
	source, err := os.Open(configFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to open source file: %v", err)
	}
	defer source.Close()

	destination, err := os.Create(backupPath)
	if err != nil {
		return "", fmt.Errorf("failed to create backup file: %v", err)
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	if err != nil {
		return "", fmt.Errorf("failed to copy file: %v", err)
	}

	return backupPath, nil
}

// ValidateConfig validates a configuration file
func ValidateConfig(configPath string) (bool, []string) {
	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return false, []string{fmt.Sprintf("Failed to read file: %v", err)}
	}

	// Try to parse as JSON
	var jsonData map[string]interface{}
	err = json.Unmarshal(data, &jsonData)
	if err != nil {
		return false, []string{fmt.Sprintf("Invalid JSON: %v", err)}
	}

	// Specific validation rules
	var issues []string

	// Check for required fields
	requiredFields := []string{"version"}
	for _, field := range requiredFields {
		if _, exists := jsonData[field]; !exists {
			issues = append(issues, fmt.Sprintf("Missing required field: %s", field))
		}
	}

	// Check for valid token format if present
	if token, exists := jsonData["bot_token"].(string); exists && token != "" {
		if !strings.HasPrefix(token, "MTA") && !strings.HasPrefix(token, "ODk") && len(token) < 50 {
			issues = append(issues, "Bot token appears to be invalid")
		}
	}

	// Check for valid channel IDs
	channelFields := []string{"newsChannelId", "auditLogChannelId", "digestChannelId", "errorChannelId"}
	for _, field := range channelFields {
		if channelID, exists := jsonData[field].(string); exists && channelID != "" {
			if len(channelID) < 17 || len(channelID) > 20 {
				issues = append(issues, fmt.Sprintf("Field %s has an invalid Discord channel ID format", field))
			}
		}
	}

	return len(issues) == 0, issues
}
