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

	// Send top positive and negative articles
	if config.TopArticlesCount > 0 {
		// Get top positive articles
		positiveArticles, err := getTopArticlesBySentiment(config.TopArticlesCount, true)
		if err == nil && len(positiveArticles) > 0 {
			embed := createTopArticlesEmbed("Most Positive Stories", positiveArticles, 0x43B581)
			s.ChannelMessageSendEmbed(channelID, embed)
		}

		// Get top negative articles
		negativeArticles, err := getTopArticlesBySentiment(config.TopArticlesCount, false)
		if err == nil && len(negativeArticles) > 0 {
			embed := createTopArticlesEmbed("Most Negative Stories", negativeArticles, 0xF04747)
			s.ChannelMessageSendEmbed(channelID, embed)
		}
	}

	// Include stats if enabled
	if config.IncludeStats {
		statsEmbed := createStatsEmbed(len(articles), len(sources))
		_, err = s.ChannelMessageSendEmbed(channelID, statsEmbed)
		if err != nil {
			Logger().Printf("Failed to send stats: %v", err)
		}
	}

	// Update state
	state.LastDigest = time.Now()
	SaveState(state)

	return nil
}

// ArticleDigest represents an article for digest
type ArticleDigest struct {
	Title     string
	URL       string
	Source    string
	Published time.Time
	Sentiment float64
	Topics    []string
}

// TrendingTopic represents a trending topic
type TrendingTopic struct {
	Topic    string
	Count    int
	Articles []string  // List of article titles
}

// getArticlesForDigest returns articles from the last N hours
func getArticlesForDigest(hours int) ([]ArticleDigest, error) {
	// If we have a database connection, use it
	if db != nil {
		query := `SELECT title, url, source_name, published_at, sentiment, category 
				FROM articles 
				WHERE published_at > datetime('now', '-' || ? || ' hours')
				ORDER BY published_at DESC`
		
		rows, err := db.Query(query, hours)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		
		var articles []ArticleDigest
		for rows.Next() {
			var article ArticleDigest
			var category string
			err := rows.Scan(&article.Title, &article.URL, &article.Source, &article.Published, 
				&article.Sentiment, &category)
			if err != nil {
				return nil, err
			}
			articles = append(articles, article)
		}
		
		return articles, nil
	}
	
	// Fallback: Use in-memory sources
	// (This is less ideal but works without database)
	return []ArticleDigest{}, nil
}

// analyzeTrendingTopics analyzes trending topics from articles
func analyzeTrendingTopics(articles []ArticleDigest) []TrendingTopic {
	// Count topic occurrences
	topicCounts := make(map[string]int)
	topicArticles := make(map[string][]string)
	
	for _, article := range articles {
		for _, topic := range article.Topics {
			topicCounts[topic]++
			
			// Add article title to this topic (avoid duplicates)
			isDuplicate := false
			for _, title := range topicArticles[topic] {
				if title == article.Title {
					isDuplicate = true
					break
				}
			}
			if !isDuplicate {
				topicArticles[topic] = append(topicArticles[topic], article.Title)
			}
		}
	}
	
	// Create trending topics
	var trending []TrendingTopic
	for topic, count := range topicCounts {
		if count >= 2 { // At least 2 mentions to be trending
			trending = append(trending, TrendingTopic{
				Topic:    topic,
				Count:    count,
				Articles: topicArticles[topic],
			})
		}
	}
	
	// Sort by count (highest first)
	sort.Slice(trending, func(i, j int) bool {
		return trending[i].Count > trending[j].Count
	})
	
	// Take top 10
	if len(trending) > 10 {
		trending = trending[:10]
	}
	
	return trending
}

// getSortedCategories returns sorted category names
func getSortedCategories(categorizedArticles map[string][]ArticleDigest) []string {
	categories := make([]string, 0, len(categorizedArticles))
	for category := range categorizedArticles {
		categories = append(categories, category)
	}
	
	// Sort categories alphabetically, but keep "General" last
	sort.Slice(categories, func(i, j int) bool {
		if categories[i] == "General" {
			return false
		}
		if categories[j] == "General" {
			return true
		}
		return categories[i] < categories[j]
	})
	
	return categories
}

// getTopArticlesBySentiment returns top positive or negative articles
func getTopArticlesBySentiment(count int, positive bool) ([]ArticleDigest, error) {
	var order string
	if positive {
		order = "DESC" // Highest sentiment first
	} else {
		order = "ASC" // Lowest sentiment first
	}
	
	query := fmt.Sprintf(`SELECT title, url, source_name, published_at, sentiment, category 
			FROM articles 
			WHERE published_at > datetime('now', '-24 hours')
			ORDER BY sentiment %s
			LIMIT ?`, order)
	
	rows, err := db.Query(query, count)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var articles []ArticleDigest
	for rows.Next() {
		var article ArticleDigest
		var category string
		err := rows.Scan(&article.Title, &article.URL, &article.Source, &article.Published, 
			&article.Sentiment, &category)
		if err != nil {
			return nil, err
		}
		articles = append(articles, article)
	}
	
	return articles, nil
}

// createTrendingEmbed creates an embed for trending topics
func createTrendingEmbed(topics []TrendingTopic) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		Title:       "📈 Trending Topics",
		Description: "Topics trending in today's news",
		Color:       0x9B59B6, // Purple
		Fields:      []*discordgo.MessageEmbedField{},
		Timestamp:   time.Now().Format(time.RFC3339),
	}
	
	if len(topics) == 0 {
		embed.Description = "No trending topics detected today."
		return embed
	}
	
	// Add trending topics as fields
	for _, topic := range topics {
		// Format article list (limited to 3)
		articleList := topic.Articles
		if len(articleList) > 3 {
			articleList = articleList[:3]
		}
		
		var articleText strings.Builder
		for i, article := range articleList {
			articleText.WriteString(fmt.Sprintf("• %s\n", truncateString(article, 60)))
			if i >= 2 && len(topic.Articles) > 3 {
				articleText.WriteString(fmt.Sprintf("• +%d more...\n", len(topic.Articles)-3))
				break
			}
		}
		
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   fmt.Sprintf("%s (%d articles)", topic.Topic, topic.Count),
			Value:  articleText.String(),
			Inline: false,
		})
	}
	
	return embed
}

// createCategoryEmbed creates an embed for a category
func createCategoryEmbed(category string, articles []ArticleDigest) *discordgo.MessageEmbed {
	// Determine color based on category
	color := 0x4B9CD3 // Default blue
	
	// Set color by category
	categoryColors := map[string]int{
		"Politics":      0x9C27B0,
		"Technology":    0x03A9F4,
		"Business":      0x4CAF50,
		"Entertainment": 0xFF9800,
		"Sports":        0xF44336,
		"Science":       0x3F51B5,
		"Health":        0x2196F3,
	}
	
	if c, ok := categoryColors[category]; ok {
		color = c
	}
	
	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("📋 %s", category),
		Description: fmt.Sprintf("Latest news in %s", category),
		Color:       color,
		Fields:      []*discordgo.MessageEmbedField{},
		Timestamp:   time.Now().Format(time.RFC3339),
	}
	
	// Add articles
	for _, article := range articles {
		// Format publish date
		publishedStr := "Unknown date"
		if !article.Published.IsZero() {
			publishedStr = fmt.Sprintf("<t:%d:R>", article.Published.Unix())
		}
		
		// Create field
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   fmt.Sprintf("%s (%s)", article.Title, article.Source),
			Value:  fmt.Sprintf("[Read Article](%s) • Published %s", article.URL, publishedStr),
			Inline: false,
		})
	}
	
	return embed
}

// createTopArticlesEmbed creates an embed for top articles by sentiment
func createTopArticlesEmbed(title string, articles []ArticleDigest, color int) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		Title:       title,
		Description: fmt.Sprintf("Notable stories in the last 24 hours"),
		Color:       color,
		Fields:      []*discordgo.MessageEmbedField{},
		Timestamp:   time.Now().Format(time.RFC3339),
	}
	
	// Add articles
	for _, article := range articles {
		// Calculate sentiment emoji based on score
		sentimentEmoji := "😐" // Neutral
		if article.Sentiment > 0.6 {
			sentimentEmoji = "😀" // Very positive
		} else if article.Sentiment > 0.2 {
			sentimentEmoji = "🙂" // Positive
		} else if article.Sentiment < -0.6 {
			sentimentEmoji = "😟" // Very negative
		} else if article.Sentiment < -0.2 {
			sentimentEmoji = "🙁" // Negative
		}
		
		// Format publish date
		publishedStr := "Unknown date"
		if !article.Published.IsZero() {
			publishedStr = fmt.Sprintf("<t:%d:R>", article.Published.Unix())
		}
		
		// Create field
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   fmt.Sprintf("%s %s (%s)", sentimentEmoji, article.Title, article.Source),
			Value:  fmt.Sprintf("[Read Article](%s) • Published %s", article.URL, publishedStr),
			Inline: false,
		})
	}
	
	return embed
}

// createStatsEmbed creates an embed with digest statistics
func createStatsEmbed(articleCount int, sourceCount int) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		Title:       "📊 Digest Statistics",
		Color:       0x607D8B, // Blue grey
		Fields:      []*discordgo.MessageEmbedField{},
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Sankarea News Bot • Daily Digest",
		},
		Timestamp:   time.Now().Format(time.RFC3339),
	}
	
	// Add stats fields
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
		Name:   "Articles Processed",
		Value:  fmt.Sprintf("%d", articleCount),
		Inline: true,
	})
	
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
		Name:   "Active Sources",
		Value:  fmt.Sprintf("%d", sourceCount),
		Inline: true,
	})
	
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
		Name:   "Next Digest",
		Value:  fmt.Sprintf("<t:%d:R>", GetNextDigestTime().Unix()),
		Inline: true,
	})
	
	return embed
}

// GetNextDigestTime calculates the next digest time based on the cron schedule
func GetNextDigestTime() time.Time {
	// Parse the cron schedule
	cronSchedule := cfg.DigestCronSchedule
	if cronSchedule == "" {
		cronSchedule = "0 8 * * *" // Default: 8 AM daily
	}
	
	// Parse the cron expression (simplified for this example)
	// In a real implementation, use a proper cron parser
	fields := strings.Fields(cronSchedule)
	if len(fields) != 5 {
		return time.Now().Add(24 * time.Hour) // Default: tomorrow
	}
	
	// Very simple parsing assuming "0 8 * * *" format (8 AM daily)
	hour := 8
	if h, err := strconv.Atoi(fields[1]); err == nil {
		hour = h
	}
	
	// Calculate next occurrence
	now := time.Now()
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, now.Location())
	
	// If today's occurrence has passed, move to tomorrow
	if next.Before(now) {
		next = next.Add(24 * time.Hour)
	}
	
	return next
}
