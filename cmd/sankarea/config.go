package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

// Config file paths
const (
	configFilePath = "config/config.json"
	sourcesFilePath = "config/sources.yml"
)

// LoadConfig loads the application configuration from disk
func LoadConfig() (*Config, error) {
	// Create config file if it doesn't exist
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		// Create default config
		defaultConfig := &Config{
			Version:             VERSION,
			NewsIntervalMinutes: 15,
			News15MinCron:       "*/15 * * * *",
			MaxPostsPerSource:   5,
			EnableImageEmbed:    true,
			UserAgentString:     "Sankarea RSS Bot v" + VERSION,
			FetchNewsOnStartup:  true,
			DigestCronSchedule:  "0 8 * * *",
			DashboardPort:       8080,
		}
		
		// Create directory if needed
		if err := os.MkdirAll(filepath.Dir(configFilePath), 0755); err != nil {
			return defaultConfig, err
		}
		
		// Save default config
		if err := SaveConfig(defaultConfig); err != nil {
			return defaultConfig, err
		}
		
		return defaultConfig, nil
	}
	
	// Read existing config file
	data, err := os.ReadFile(configFilePath)
	if err != nil {
		return nil, err
	}
	
	// Parse config
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	
	// Load environment variables (overrides file settings)
	config.BotToken = getEnvOrValue(config.BotToken, "DISCORD_BOT_TOKEN")
	config.AppID = getEnvOrValue(config.AppID, "DISCORD_APPLICATION_ID")
	config.GuildID = getEnvOrValue(config.GuildID, "DISCORD_GUILD_ID")
	config.NewsChannelID = getEnvOrValue(config.NewsChannelID, "DISCORD_CHANNEL_ID")
	
	// If version is missing, set it
	if config.Version == "" {
		config.Version = VERSION
	}
	
	// Enforce minimum values
	if config.NewsIntervalMinutes < 5 {
		config.NewsIntervalMinutes = 5
	}
	if config.MaxPostsPerSource < 1 {
		config.MaxPostsPerSource = 5
	}
	
	return &config, nil
}

// SaveConfig saves the application configuration to disk
func SaveConfig(cfg *Config) error {
	// Marshal config to JSON
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	
	// Create directory if needed
	if err := os.MkdirAll(filepath.Dir(configFilePath), 0755); err != nil {
		return err
	}
	
	// Save config
	return os.WriteFile(configFilePath, data, 0644)
}

// getEnvOrValue returns the environment variable value or the default value
func getEnvOrValue(defaultValue, envKey string) string {
	if value, exists := os.LookupEnv(envKey); exists && value != "" {
		return value
	}
	return defaultValue
}

// LoadSources loads news sources from disk
func LoadSources() ([]Source, error) {
	// Check if file exists, create if not
	if _, err := os.Stat(sourcesFilePath); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(sourcesFilePath), 0755); err != nil {
			return nil, err
		}
		
		// Create empty file with empty YAML sources array
		if err := os.WriteFile(sourcesFilePath, []byte("sources: []"), 0644); err != nil {
			return nil, err
		}
		
		return []Source{}, nil
	}
	
	// Read sources file
	data, err := os.ReadFile(sourcesFilePath)
	if err != nil {
		return nil, err
	}
	
	var sources struct {
		Sources []Source `yaml:"sources"`
	}
	
	if err := yaml.Unmarshal(data, &sources); err != nil {
		// Try with direct unmarshaling if the structure doesn't match
		var directSources []Source
		if err := yaml.Unmarshal(data, &directSources); err != nil {
			return nil, fmt.Errorf("failed to parse sources file: %w", err)
		}
		sources.Sources = directSources
	}
	
	// Ensure all sources have active flag set properly
	for i := range sources.Sources {
		if !sources.Sources[i].Paused && sources.Sources[i].Active == false {
			sources.Sources[i].Active = true
		}
	}
	
	return sources.Sources, nil
}

// SaveSources saves news sources to disk
func SaveSources(sources []Source) error {
	// Create wrapper structure
	wrapper := struct {
		Sources []Source `yaml:"sources"`
	}{
		Sources: sources,
	}
	
	// Marshal to YAML
	data, err := yaml.Marshal(wrapper)
	if err != nil {
		return err
	}
	
	// Create directory if needed
	if err := os.MkdirAll(filepath.Dir(sourcesFilePath), 0755); err != nil {
		return err
	}
	
	// Save file
	return os.WriteFile(sourcesFilePath, data, 0644)
}

// AddOrUpdateSource adds a new source or updates an existing one
func AddOrUpdateSource(source Source) error {
	sources, err := LoadSources()
	if err != nil {
		return err
	}
	
	// Check if source already exists
	found := false
	for i, s := range sources {
		if s.Name == source.Name {
			sources[i] = source
			found = true
			break
		}
	}
	
	// Add new source if not found
	if !found {
		sources = append(sources, source)
	}
	
	return SaveSources(sources)
}

// RemoveSource removes a source by name
func RemoveSource(name string) error {
	sources, err := LoadSources()
	if err != nil {
		return err
	}
	
	// Find and remove source
	for i, s := range sources {
		if s.Name == name {
			// Remove this source
			sources = append(sources[:i], sources[i+1:]...)
			break
		}
	}
	
	return SaveSources(sources)
}

// UpdateSourceStatus updates a source's paused status
func UpdateSourceStatus(name string, paused bool) error {
	sources, err := LoadSources()
	if err != nil {
		return err
	}
	
	// Find and update source
	found := false
	for i, s := range sources {
		if s.Name == name {
			sources[i].Paused = paused
			found = true
			break
		}
	}
	
	if !found {
		return fmt.Errorf("source %s not found", name)
	}
	
	return SaveSources(sources)
}
