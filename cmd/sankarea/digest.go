package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

// DigestConfig holds configuration for news digests
type DigestConfig struct {
	Categories       []string // Categories to include
	MaxItemsPerCategory int   // Max items per category
	TopArticlesCount int      // Number of top articles to show
	IncludeTrending  bool     // Whether to include trending analysis
	IncludeStats     bool     // Whether to include stats
}

// GenerateDailyDigest creates a daily news digest
func GenerateDailyDigest(s *discordgo.Session, channelID string) error {
	// Load sources
	sources, err := LoadSources()
	if err != nil {
		return fmt.Errorf("Failed to load sources: %v", err)
	}

	// Load state
	state, err := LoadState()
	if err != nil {
		return fmt.Errorf("Failed to load state: %v", err)
	}

	// Default digest config
	config := DigestConfig{
		Categories:        []string{},  // All categories
		MaxItemsPerCategory: 5,
		TopArticlesCount:  3,
		IncludeTrending:   true,
		IncludeStats:      true,
	}

	// Get today's date
	today := time.Now().Format("Monday, January 2, 2006")

	// Send digest header
	_, err = s.ChannelMessageSend(channelID, fmt.Sprintf("# 📰 Daily News Digest for %s", today))
	if err != nil {
		return fmt.Errorf("Failed to send header: %v", err)
	}

	// Get articles from the database
	articles, err := getArticlesForDigest(24) // Last 24 hours
	if err != nil {
		return fmt.Errorf("Failed to retrieve articles: %v", err)
	}

	// Group articles by category
	categorizedArticles := make(map[string][]ArticleDigest)
	for _, article := range articles {
		cat := article.Category
		if cat == "" {
			cat = "General"
		}
		categorizedArticles[cat] = append(categorizedArticles[cat], article)
	}

	// Generate trending topics
	if config.IncludeTrending {
		trendingTopics := analyzeTrendingTopics(articles)
		embed := createTrendingEmbed(trendingTopics)
		_, err = s.ChannelMessageSendEmbed(channelID, embed)
		if err != nil {
			Logger().Printf("Failed to send trending topics: %v", err)
		}
	}

	// Send each category
	categories := getSortedCategories(categorizedArticles)
	for _, category := range categories {
		articles := categorizedArticles[category]
		if len(articles) == 0 {
			continue
		}

		// Sort articles by published time (newest first)
		sort.Slice(articles, func(i, j int) bool {
			return articles[i].Published.After(articles[j].Published)
		})

		// Limit articles per category
		if len(articles) > config.MaxItemsPerCategory {
			articles = articles[:config.MaxItemsPerCategory]
		}

		// Create category embed
		embed := createCategoryEmbed(category, articles)
		_, err = s.ChannelMessageSendEmbed(channelID, embed)
		if err != nil {
			Logger().Printf("Failed to send category digest: %v", err)
		}

		// Small delay to avoid rate limiting
		time.Sleep(500 * time.Millisecond)
	}

	// Send stats if requested
	if config.IncludeStats {
		statsEmbed := createStatsEmbed(sources)
		_, err = s.ChannelMessageSendEmbed(channelID, statsEmbed)
		if err != nil {
			Logger().Printf("Failed to send stats: %v", err)
		}
	}

	// Update state
	state.DigestCount++
	state.DigestNextTime = calculateNextDigestTime(cfg.DigestCronSchedule)
	SaveState(state)

	return nil
}

// ArticleDigest represents an article in a digest
type ArticleDigest struct {
	Title     string
	URL       string
	Source    string
	Published time.Time
	Category  string
	Bias      string
	Summary   string
}

// getArticlesForDigest gets articles for a digest
// This would normally pull from a database, but for this example we'll simulate
func getArticlesForDigest(hours int) ([]ArticleDigest, error) {
	// In a real implementation, this would fetch from a database
	// For now, return sample data
	return []ArticleDigest{
		{
			Title:     "Global Economy Shows Signs of Recovery",
			URL:       "https://example.com/economy-recovery",
			Source:    "Reuters",
			Published: time.Now().Add(-3 * time.Hour),
			Category:  "Business",
			Bias:      "Center",
		},
		{
			Title:     "New Climate Agreement Reached",
			URL:       "https://example.com/climate-agreement",
			Source:    "BBC",
			Published: time.Now().Add(-5 * time.Hour),
			Category:  "Environment",
			Bias:      "Center-Left",
		},
		{
			Title:     "Tech Companies Face New Regulations",
			URL:       "https://example.com/tech-regulations",
			Source:    "CNN",
			Published: time.Now().Add(-8 * time.Hour),
			Category:  "Technology",
			Bias:      "Left-Center",
		},
		{
			Title:     "Sports Team Wins Championship",
			URL:       "https://example.com/sports-championship",
			Source:    "Fox News",
			Published: time.Now().Add(-12 * time.Hour),
			Category:  "Sports",
			Bias:      "Right",
		},
	}, nil
}

// analyzeTrendingTopics analyzes articles to find trending topics
func analyzeTrendingTopics(articles []ArticleDigest) []string {
	// In a real implementation, this would do NLP analysis
	// For now, return sample data
	return []string{
		"Climate Change",
		"Economic Recovery",
		"Tech Regulation",
	}
}

// getSortedCategories returns sorted category names
func getSortedCategories(categorizedArticles map[string][]ArticleDigest) []string {
	var categories []string
	for cat := range categorizedArticles {
		categories = append(categories, cat)
	}
	sort.Strings(categories)
	return categories
}

// createTrendingEmbed creates an embed for trending topics
func createTrendingEmbed(topics []string) *discordgo.MessageEmbed {
	description := "Current trending topics in the news:"
	for _, topic := range topics {
		description += fmt.Sprintf("\n• **%s**", topic)
	}

	return &discordgo.MessageEmbed{
		Title:       "🔥 Trending Topics",
		Description: description,
		Color:       0xFF9900, // Orange
	}
}

// createCategoryEmbed creates an embed for a category
func createCategoryEmbed(category string, articles []ArticleDigest) *discordgo.MessageEmbed {
	// Set color based on category
	color := 0x0099FF // Default blue
	switch strings.ToLower(category) {
	case "politics":
		color = 0x880088 // Purple
	case "business", "economy":
		color = 0x008800 // Green
	case "technology":
		color = 0x0000FF // Blue
	case "health":
		color = 0xFF0000 // Red
	case "science":
		color = 0x00FFFF // Cyan
	case "entertainment":
		color = 0xFF00FF // Pink
	case "sports":
		color = 0xFF8800 // Orange
	}

	// Build description
	description := ""
	for _, article := range articles {
		description += fmt.Sprintf("• [%s](%s) - %s (%s)\n", 
			article.Title, 
			article.URL, 
			article.Source,
			article.Published.Format("Jan 2, 15:04"))
	}

	return &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("📰 %s News", category),
		Description: description,
		Color:       color,
	}
}

// createStatsEmbed creates an embed with stats
func createStatsEmbed(sources []Source) *discordgo.MessageEmbed {
	// Count active sources by bias
	biasCounts := make(map[string]int)
	totalSources := 0
	activeSources := 0

	for _, source := range sources {
		totalSources++
		if source.Active && !source.Paused {
			activeSources++
			bias := source.Bias
			if bias == "" {
				bias = "Unknown"
			}
			biasCounts[bias]++
		}
	}

	// Build description
	description := fmt.Sprintf("**Active Sources**: %d/%d\n\n**Coverage by Bias**:\n", 
		activeSources, totalSources)
	
	// Add bias distribution
	biases := []string{"Left", "Left-Center", "Center", "Right-Center", "Right"}
	for _, bias := range biases {
		count := biasCounts[bias]
		bar := strings.Repeat("█", count)
		if bar == "" {
			bar = "▫️"
		}
		description += fmt.Sprintf("%s: %s (%d)\n", bias, bar, count)
	}

	// Add unknown if any
	if count := biasCounts["Unknown"]; count > 0 {
		description += fmt.Sprintf("Unknown: %s (%d)\n", 
			strings.Repeat("█", count), count)
	}

	return &discordgo.MessageEmbed{
		Title:       "📊 News Coverage Stats",
		Description: description,
		Color:       0x888888, // Gray
	}
}

// calculateNextDigestTime calculates when the next digest should run
func calculateNextDigestTime(cronSchedule string) time.Time {
	// This would normally use proper cron parsing
	// For simplicity, default to next day at 8 AM
	now := time.Now()
	tomorrow := now.AddDate(0, 0, 1)
	return time.Date(
		tomorrow.Year(),
		tomorrow.Month(),
		tomorrow.Day(),
		8, 0, 0, 0,
		tomorrow.Location(),
	)
}
