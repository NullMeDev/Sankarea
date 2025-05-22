package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// ThemeColors defines color scheme for embeds
type ThemeColors struct {
	Primary       int    `json:"primary"`       // Default embed color
	Success       int    `json:"success"`       // Success messages
	Error         int    `json:"error"`         // Errors
	Warning       int    `json:"warning"`       // Warnings
	Info          int    `json:"info"`          // Information
	Politics      int    `json:"politics"`      // Politics category
	Business      int    `json:"business"`      // Business category
	Technology    int    `json:"technology"`    // Technology category
	Entertainment int    `json:"entertainment"` // Entertainment category
	Sports        int    `json:"sports"`        // Sports category
	Health        int    `json:"health"`        // Health category
	Science       int    `json:"science"`       // Science category
	LeftBias      int    `json:"leftBias"`      // Left-leaning sources
	RightBias     int    `json:"rightBias"`     // Right-leaning sources
	CenterBias    int    `json:"centerBias"`    // Center sources
	ThemeName     string `json:"themeName"`     // Name of the theme
}

// Layout defines layout settings
type Layout struct {
	CompactPosts        bool `json:"compactPosts"`        // Use compact post format
	GroupByCategory     bool `json:"groupByCategory"`     // Group posts by category
	ShowSourceImages    bool `json:"showSourceImages"`    // Show source thumbnails
	ShowFullTimestamps  bool `json:"showFullTimestamps"`  // Show full timestamps vs relative time
	MaxPostsPerSource   int  `json:"maxPostsPerSource"`   // Max posts to show per source
	MaxDigestItems      int  `json:"maxDigestItems"`      // Max items in digest
	DigestGroupBySource bool `json:"digestGroupBySource"` // Group digest by source rather than category
}

// Theme represents a complete theme configuration
type Theme struct {
	Colors      ThemeColors `json:"colors"`
	Layout      Layout      `json:"layout"`
	Description string      `json:"description"`
}

var (
	activeTheme   Theme
	defaultTheme  Theme
	themesDir     = "config/themes"
	activeThemeID = "default"
)

// InitThemeManager initializes the theme manager
func InitThemeManager() error {
	// Create themes directory if it doesn't exist
	if _, err := os.Stat(themesDir); os.IsNotExist(err) {
		if err := os.MkdirAll(themesDir, 0755); err != nil {
			return fmt.Errorf("failed to create themes directory: %v", err)
		}
	}
	
	// Set up default theme
	defaultTheme = Theme{
		Description: "Default Sankarea theme",
		Colors: ThemeColors{
			Primary:       0x0099FF, // Blue
			Success:       0x00CC00, // Green
			Error:         0xFF0000, // Red
			Warning:       0xFFCC00, // Yellow
			Info:          0x00AAFF, // Light blue
			Politics:      0x880088, // Purple
			Business:      0x008800, // Green
			Technology:    0x0000FF, // Blue
			Entertainment: 0xFF00FF, // Pink
			Sports:        0xFF8800, // Orange
			Health:        0xFF0000, // Red
			Science:       0x00FFFF, // Cyan
			LeftBias:      0x0000FF, // Blue
			RightBias:     0xFF0000, // Red
			CenterBias:    0x808080, // Gray
			ThemeName:     "Default",
		},
		Layout: Layout{
			CompactPosts:        false,
			GroupByCategory:     false,
			ShowSourceImages:    true,
			ShowFullTimestamps:  false,
			MaxPostsPerSource:   5,
			MaxDigestItems:      5,
			DigestGroupBySource: false,
		},
	}
	
	// Save default theme if it doesn't exist
	defaultThemePath := filepath.Join(themesDir, "default.json")
	if _, err := os.Stat(defaultThemePath); os.IsNotExist(err) {
		if err := SaveTheme("default", &defaultTheme); err != nil {
			return err
		}
	}
	
	// Initialize dark theme
	darkTheme := Theme{
		Description: "Dark theme with muted colors",
		Colors: ThemeColors{
			Primary:       0x2C3E50, // Dark blue-gray
			Success:       0x27AE60, // Dark green
			Error:         0xC0392B, // Dark red
			Warning:       0xF39C12, // Dark yellow
			Info:          0x2980B9, // Dark blue
			Politics:      0x8E44AD, // Dark purple
			Business:      0x16A085, // Dark teal
			Technology:    0x2980B9, // Dark blue
			Entertainment: 0xD35400, // Dark orange
			Sports:        0xE67E22, // Dark orange
			Health:        0xE74C3C, // Dark red
			Science:       0x1ABC9C, // Dark cyan
			LeftBias:      0x3498DB, // Dark blue
			RightBias:     0xE74C3C, // Dark red
			CenterBias:    0x7F8C8D, // Dark gray
			ThemeName:     "Dark",
		},
		Layout: Layout{
			CompactPosts:        true,
			GroupByCategory:     true,
			ShowSourceImages:    false,
			ShowFullTimestamps:  true,
			MaxPostsPerSource:   3,
			MaxDigestItems:      10,
			DigestGroupBySource: true,
		},
	}
	
	// Save dark theme if it doesn't exist
	darkThemePath := filepath.Join(themesDir, "dark.json")
	if _, err := os.Stat(darkThemePath); os.IsNotExist(err) {
		if err := SaveTheme("dark", &darkTheme); err != nil {
			return err
		}
	}
	
	// Load default theme
	if err := LoadTheme("default"); err != nil {
		return err
	}
	
	return nil
}

// SaveTheme saves a theme to a file
func SaveTheme(name string, theme *Theme) error {
	// Ensure name is set in theme
	theme.Colors.ThemeName = name
	
	// Convert to JSON
	data, err := json.MarshalIndent(theme, "", "  ")
	if err != nil {
		return err
	}
	
	// Save to file
	path := filepath.Join(themesDir, name+".json")
	return ioutil.WriteFile(path, data, 0644)
}

// LoadTheme loads a theme from a file
func LoadTheme(name string) error {
	path := filepath.Join(themesDir, name+".json")
	
	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("theme '%s' does not exist", name)
	}
	
	// Read file
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	
	// Parse JSON
	var theme Theme
	if err := json.Unmarshal(data, &theme); err != nil {
		return err
	}
	
	// Set as active theme
	activeTheme = theme
	activeThemeID = name
	
	Logger().Printf("Loaded theme: %s", name)
	return nil
}

// GetThemeColor returns the color for a specific content type
func GetThemeColor(contentType string, category string, bias string) int {
	// Check if active theme is initialized
	if activeTheme.Colors.ThemeName == "" {
		return defaultTheme.Colors.Primary // Fallback
	}
	
	// Select color based on content type
	switch contentType {
	case "success":
		return activeTheme.Colors.Success
	case "error":
		return activeTheme.Colors.Error
	case "warning":
		return activeTheme.Colors.Warning
	case "info":
		return activeTheme.Colors.Info
	case "category":
		// Select color based on category
		switch category {
		case "Politics":
			return activeTheme.Colors.Politics
		case "Business", "Economy":
			return activeTheme.Colors.Business
		case "Technology":
			return activeTheme.Colors.Technology
		case "Entertainment":
			return activeTheme.Colors.Entertainment
		case "Sports":
			return activeTheme.Colors.Sports
		case "Health":
			return activeTheme.Colors.Health
		case "Science":
			return activeTheme.Colors.Science
		default:
			return activeTheme.Colors.Primary
		}
	case "bias":
		// Select color based on bias
		if strings.Contains(bias, "Left") {
			return activeTheme.Colors.LeftBias
		} else if strings.Contains(bias, "Right") {
			return activeTheme.Colors.RightBias
		} else {
			return activeTheme.Colors.CenterBias
		}
	default:
		return activeTheme.Colors.Primary
	}
}

// GetActiveTheme returns the current active theme
func GetActiveTheme() *Theme {
	return &activeTheme
}

// ListAvailableThemes returns a list of available themes
func ListAvailableThemes() ([]string, error) {
	// Get all files in themes directory
	files, err := ioutil.ReadDir(themesDir)
	if err != nil {
		return nil, err
	}
	
	// Extract theme names
	var themes []string
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".json" {
			themes = append(themes, file.Name()[:len(file.Name())-5]) // Remove .json extension
		}
	}
	
	return themes, nil
}
