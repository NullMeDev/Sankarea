package main

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mmcdole/gofeed"
)

// Article represents a news article from a feed
type Article struct {
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Content     string    `json:"content"`
	Link        string    `json:"link"`
	Published   time.Time `json:"published"`
	Updated     time.Time `json:"updated"`
	GUID        string    `json:"guid"`
	ImageURL    string    `json:"imageUrl"`
	Categories  []string  `json:"categories"`
	Source      string    `json:"source"`
	SourceURL   string    `json:"sourceUrl"`
}

// fetchAndPostNews fetches news from all sources and posts them to the specified channel
func fetchAndPostNews(s *discordgo.Session, channelID string, sources []Source) error {
	if s == nil {
		return fmt.Errorf("discord session is nil")
	}

	if channelID == "" {
		return fmt.Errorf("channel ID is empty")
	}

	if len(sources) == 0 {
		Logger().Println("No sources configured, skipping news fetch")
		return nil
	}

	// Process each source in parallel with a limit on concurrency
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 5) // Limit to 5 concurrent fetches
	errors := make(chan error, len(sources))

	for _, source := range sources {
		// Skip paused sources
		if source.Paused {
			Logger().Printf("Skipping paused source: %s", source.Name)
			continue
		}

		wg.Add(1)
		go func(src Source) {
			defer wg.Done()
			defer RecoverFromPanic(fmt.Sprintf("fetch-news-%s", src.Name))

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Use the channel override if specified
			targetChannel := channelID
			if src.ChannelOverride != "" {
				targetChannel = src.ChannelOverride
			}

			// Fetch and post news for this source
			if err := fetchSourceNews(s, targetChannel, src); err != nil {
				errors <- fmt.Errorf("error fetching news from %s: %w", src.Name, err)
				
				// Update source with error information
				updateSourceWithError(src, err)
			}
		}(source)
	}

	// Wait for all fetches to complete
	wg.Wait()
	close(errors)

	// Collect errors
	var errs []string
	for err := range errors {
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors fetching news: %s", strings.Join(errs, "; "))
	}

	return nil
}

// fetchSourceNews fetches news from a single source and posts them to the specified channel
func fetchSourceNews(s *discordgo.Session, channelID string, source Source) error {
	Logger().Printf("Fetching news from %s (%s)", source.Name, source.URL)
	
	// Record start time for metrics
	startTime := time.Now()
	
	// Parse feed
	fp := gofeed.NewParser()
	fp.UserAgent = cfg.UserAgentString
	
	feed, err := fp.ParseURL(source.URL)
	if err != nil {
		return fmt.Errorf("failed to parse feed: %w", err)
	}
	
	// Process feed items
	var articles []*Article
	for _, item := range feed.Items {
		// Skip items with no title or link
		if item.Title == "" || item.Link == "" {
			continue
		}
		
		// Convert to our article format
		article := &Article{
			Title:       item.Title,
			Description: item.Description,
			Content:     item.Content,
			Link:        item.Link,
			GUID:        item.GUID,
			Source:      source.Name,
			SourceURL:   source.URL,
			Categories:  item.Categories,
		}
		
		// Set published time
		if item.PublishedParsed != nil {
			article.Published = *item.PublishedParsed
		} else if item.UpdatedParsed != nil {
			article.Published = *item.UpdatedParsed
		} else {
			article.Published = time.Now()
		}
		
		// Set updated time
		if item.UpdatedParsed != nil {
			article.Updated = *item.UpdatedParsed
		} else {
			article.Updated = article.Published
		}
		
		// Extract image URL
		if item.Image != nil && item.Image.URL != "" {
			article.ImageURL = item.Image.URL
		} else {
			// Try to extract from content or description
			article.ImageURL = extractImageURL(item.Content)
			if article.ImageURL == "" {
				article.ImageURL = extractImageURL(item.Description)
			}
		}
		
		articles = append(articles, article)
	}
	
	// Sort articles by published date (newest first)
	sortArticlesByDate(articles)
	
	// Limit to configured number of posts per source
	maxPosts := cfg.MaxPostsPerSource
	if maxPosts <= 0 {
		maxPosts = 5 // Default to 5 if not set
	}
	
	if len(articles) > maxPosts {
		articles = articles[:maxPosts]
	}
	
	// Post articles to Discord
	for _, article := range articles {
		if err := postArticleToDiscord(s, channelID, article, source); err != nil {
			Logger().Printf("Error posting article to Discord: %v", err)
			continue
		}
		
		// Increment feed count
		IncrementFeedCount()
		
		// Rate limit to avoid Discord API rate limits
		time.Sleep(500 * time.Millisecond)
	}
	
	// Update metrics for this source
	updateSourceMetrics(source, startTime, len(articles), nil)
	
	return nil
}

// extractImageURL extracts the first image URL from HTML content
func extractImageURL(content string) string {
	// Simple regex for extracting image URL would be used here
	// For brevity, we'll just use a placeholder implementation
	if strings.Contains(content, "<img") && strings.Contains(content, "src=\"") {
		start := strings.Index(content, "src=\"") + 5
		end := strings.Index(content[start:], "\"")
		if end > 0 {
			return content[start : start+end]
		}
	}
	return ""
}

// sortArticlesByDate sorts articles by published date (newest first)
func sortArticlesByDate(articles []*Article) {
	sort.Slice(articles, func(i, j int) bool {
		return articles[i].Published.After(articles[j].Published)
	})
}

// postArticleToDiscord posts an article to a Discord channel
func postArticleToDiscord(s *discordgo.Session, channelID string, article *Article, source Source) error {
	// Create embed
	embed := &discordgo.MessageEmbed{
		Title:       article.Title,
		Description: truncateDescription(article.Description, 300),
		URL:         article.Link,
		Color:       getColorForCategory(source.Category),
		Timestamp:   article.Published.Format(time.RFC3339),
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("Source: %s", source.Name),
		},
	}
	
	// Add image if available
	if article.ImageURL != "" && cfg.EnableImageEmbed {
		embed.Image = &discordgo.MessageEmbedImage{
			URL: article.ImageURL,
		}
	}
	
	// Add categories as fields if available
	if len(article.Categories) > 0 {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "Categories",
			Value:  strings.Join(article.Categories, ", "),
			Inline: true,
		})
	}
	
	// Add credibility score if available
	if source.TrustScore > 0 {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "Trust Score",
			Value:  fmt.Sprintf("%.1f/10", source.TrustScore),
			Inline: true,
		})
	}
	
	// Post embed to Discord
	_, err := s.ChannelMessageSendEmbed(channelID, embed)
	return err
}

// truncateDescription truncates a string to the specified length and adds ellipsis if needed
func truncateDescription(s string, maxLength int) string {
	// Replace HTML tags with plain text
	s = strings.ReplaceAll(s, "<br>", "\n")
	s = strings.ReplaceAll(s, "<br/>", "\n")
	s = strings.ReplaceAll(s, "<br />", "\n")
	
	// Use SanitizeHTML for proper HTML removal
	s = SanitizeHTML(s)
	
	// Truncate if needed
	return TruncateString(s, maxLength)
}

// getColorForCategory returns a color code based on the category
func getColorForCategory(category string) int {
	switch strings.ToLower(category) {
	case "technology":
		return 0x3498db // Blue
	case "business":
		return 0x2ecc71 // Green
	case "politics":
		return 0xe74c3c // Red
	case "entertainment":
		return 0x9b59b6 // Purple
	case "sports":
		return 0xf39c12 // Orange
	case "science":
		return 0x1abc9c // Teal
	case "health":
		return 0x2980b9 // Dark Blue
	default:
		return 0x95a5a6 // Gray
	}
}

// updateSourceWithError updates a source with error information
func updateSourceWithError(source Source, err error) {
	// Update source error information
	source.LastError = err.Error()
	source.LastErrorTime = time.Now()
	source.ErrorCount++
	
	// Load all sources, update this one, and save
	sources, loadErr := LoadSources()
	if loadErr != nil {
		Logger().Printf("Failed to load sources for error update: %v", loadErr)
		return
	}
	
	// Find and update this source
	for i, src := range sources {
		if src.Name == source.Name {
			source.FeedCount = src.FeedCount
			source.UptimePercent = src.UptimePercent
			source.AvgResponseTime = src.AvgResponseTime
			sources[i] = source
			break
		}
	}
	
	// Save updated sources
	if saveErr := SaveSources(sources); saveErr != nil {
		Logger().Printf("Failed to save sources after error update: %v", saveErr)
	}
}

// updateSourceMetrics updates metrics for a source
func updateSourceMetrics(source Source, startTime time.Time, articleCount int, err error) {
	// Calculate response time
	responseTime := time.Since(startTime).Milliseconds()
	
	// Update source metrics
	source.LastFetched = time.Now()
	source.AvgResponseTime = (source.AvgResponseTime + responseTime) / 2
	source.FeedCount += articleCount
	
	// Update uptime percentage
	if err == nil {
		// Successful fetch
		source.UptimePercent = (source.UptimePercent*0.9 + 100*0.1) // Weighted average
	} else {
		// Failed fetch
		source.UptimePercent = source.UptimePercent * 0.9 // Decrease uptime
	}
	
	// Load all sources, update this one, and save
	sources, loadErr := LoadSources()
	if loadErr != nil {
		Logger().Printf("Failed to load sources for metrics update: %v", loadErr)
		return
	}
	
	// Find and update this source
	for i, src := range sources {
		if src.Name == source.Name {
			sources[i] = source
			break
		}
	}
	
	// Save updated sources
	if saveErr := SaveSources(sources); saveErr != nil {
		Logger().Printf("Failed to save sources after metrics update: %v", saveErr)
	}
	
	// Update global article count
	UpdateTotalArticles(articleCount)
}
