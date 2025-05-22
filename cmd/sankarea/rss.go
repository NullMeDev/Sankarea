package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mmcdole/gofeed"
)

// fetchAndPostNews retrieves each feed and posts a simple report
func fetchAndPostNews(s *discordgo.Session, channelID string, sources []Source) {
	state, err := LoadState()
	if err != nil {
		Logger().Printf("Cannot load state: %v", err)
		return
	}

	if state.Paused {
		Logger().Println("News fetch paused by system state")
		return
	}

	parser := gofeed.NewParser()
	sourcesUpdated := false
	articlesProcessed := 0
	
	// Track which articles we've already sent (by URL)
	sentArticles := make(map[string]bool)
	
	for i, src := range sources {
		if src.Paused || !src.Active {
			continue
		}

		// Implement rate limiting and retry logic
		var feed *gofeed.Feed
		var err error
		
		feed, err = fetchFeedWithRetry(parser, src.URL, cfg.MaxRetryCount, time.Duration(cfg.RetryDelaySeconds)*time.Second)

		if err != nil {
			Logger().Printf("fetch %s failed after %d retries: %v", src.Name, cfg.MaxRetryCount, err)
			sources[i].LastError = err.Error()
			sources[i].LastErrorTime = time.Now()
			sources[i].ErrorCount++
			sourcesUpdated = true
			continue
		}
		
		// Check language filter if enabled
		if cfg.EnableMultiLanguage && src.Language != "" && len(cfg.SupportedLanguages) > 0 {
			supportedLanguage := false
			for _, lang := range cfg.SupportedLanguages {
				if src.Language == lang {
					supportedLanguage = true
					break
				}
			}
			
			if !supportedLanguage {
				Logger().Printf("Skipping source %s due to unsupported language: %s", src.Name, src.Language)
				continue
			}
		}

		// Check if we have items to post
		if len(feed.Items) > 0 {
			// Format the message
			msg := fmt.Sprintf("🔗 **%s**: %s\n", src.Name, feed.Title)
			
			// Limit the number of posts
			maxPosts := cfg.MaxPostsPerSource
			if maxPosts <= 0 {
				maxPosts = 5
			}
			
			postCount := 0
			for _, item := range feed.Items {
				if postCount >= maxPosts {
					break
				}
				
				// Only include items with a valid published date
				if item.PublishedParsed != nil {
					// Skip if we've already sent this article
					if item.Link != "" && sentArticles[item.Link] {
						continue
					}
					
					// Store the fact we're sending this article
					if item.Link != "" {
						sentArticles[item.Link] = true
					}
					
					// Format and send message
					msg += fmt.Sprintf("• [%s](%s) - %s\n", 
						item.Title,
						item.Link, 
						item.PublishedParsed.Format("Jan 02"))
					postCount++
					articlesProcessed++
					
					// Process keywords
					if cfg.EnableKeywordTracking && keywordTracker != nil {
						go keywordTracker.CheckForKeywords(item.Title + " " + item.Description)
					}
					
					// Auto fact-check if enabled for this source
					if cfg.EnableFactCheck && src.FactCheckAuto && item.Link != "" {
						go performAutoFactCheck(s, item, src)
					}
					
					// Auto summarize if enabled for this source
					if cfg.EnableSummarization && src.SummarizeAuto && item.Link != "" {
						go performAutoSummarize(s, item, src)
					}
					
					// Track article for analytics
					if cfg.EnableAnalytics && analyticsEngine != nil {
						go analyticsEngine.TrackArticle(item, src)
					}
				}
			}
			
			// Only send if we have articles to post
			if postCount > 0 {
				// Use channel override if specified
				postChannelID := channelID
				if src.ChannelOverride != "" {
					postChannelID = src.ChannelOverride
				}
				
				// Send the message
				_, err = s.ChannelMessageSend(postChannelID, msg)
				if err != nil {
					Logger().Printf("Failed to send message: %v", err)
				}
			}
			
			sources[i].FeedCount += postCount
			sources[i].LastFetched = time.Now()
			sources[i].LastError = ""
			sourcesUpdated = true
		}
	}
	
	// Update the next time in the state
	state.NewsNextTime = time.Now().Add(parseCron(cfg.News15MinCron))
	state.LastInterval = int(parseCron(cfg.News15MinCron).Minutes())
	state.LastFetchTime = time.Now()
	state.TotalArticles += articlesProcessed
	SaveState(state)
	
	// Save sources if they were updated
	if sourcesUpdated {
		SaveSources(sources)
	}
	
	Logger().Printf("News fetch completed: processed %d articles", articlesProcessed)
}

// fetchFeedWithRetry attempts to fetch an RSS feed with retries
func fetchFeedWithRetry(parser *gofeed.Parser, url string, maxRetries int, delay time.Duration) (*gofeed.Feed, error) {
	var feed *gofeed.Feed
	var err error
	
	for i := 0; i <= maxRetries; i++ {
		feed, err = parser.ParseURL(url)
		if err == nil {
			return feed, nil
		}
		
		if i < maxRetries {
			Logger().Printf("Retry %d/%d for URL %s after error: %v", i+1, maxRetries, url, err)
			time.Sleep(delay)
		}
	}
	
	return nil, err
}

// parseCron parses a cron expression and returns the duration to the next run
func parseCron(cronExpr string) time.Duration {
	// Default to 15 minutes if parsing fails
	defaultDuration := 15 * time.Minute
	
	if cronExpr == "" {
		return defaultDuration
	}
	
	// Handle simple cron format for fixed intervals: */15 * * * *
	if strings.HasPrefix(cronExpr, "*/") {
		parts := strings.Split(cronExpr, " ")
		if len(parts) == 5 && strings.HasPrefix(parts[0], "*/") {
			minuteStr := strings.TrimPrefix(parts[0], "*/")
			minutes, err := strconv.Atoi(minuteStr)
			if err == nil && minutes > 0 {
				return time.Duration(minutes) * time.Minute
			}
		}
	}
	
	return defaultDuration
}

// performAutoFactCheck performs fact checking on an article
func performAutoFactCheck(s *discordgo.Session, item *gofeed.Item, source Source) {
	// Only proceed if we have the required API keys
	if cfg.OpenAIAPIKey == "" && cfg.GoogleFactCheckAPIKey == "" && cfg.ClaimBustersAPIKey == "" {
		return
	}
	
	// Extract article content
	articleContent := ""
	if item.Description != "" {
		articleContent = item.Description
	} else if item.Content != "" {
		articleContent = item.Content
	}
	
	// Perform fact check
	factCheck, err := FactCheckArticle(item.Title, articleContent, item.Link)
	if err != nil {
		Logger().Printf("Auto fact check failed for %s: %v", item.Link, err)
		return
	}
	
	// Only report significant fact check results (low trust score)
	if factCheck.TrustScore < 0.7 {
		// Send fact check result to audit log channel
		if cfg.AuditLogChannelID != "" {
			embed := createFactCheckEmbed(factCheck, item, source)
			_, err := s.ChannelMessageSendEmbed(cfg.AuditLogChannelID, embed)
			if err != nil {
				Logger().Printf("Failed to send fact check result: %v", err)
			}
		}
	}
}

// createFactCheckEmbed creates a Discord embed for fact check results
func createFactCheckEmbed(factCheck *FactCheckResult, item *gofeed.Item, source Source) *discordgo.MessageEmbed {
	// Determine color based on trust score
	var color int
	if factCheck.TrustScore > 0.7 {
		color = 0x00FF00 // Green
	} else if factCheck.TrustScore > 0.4 {
		color = 0xFFFF00 // Yellow
	} else {
		color = 0xFF0000 // Red
	}
	
	// Format trust score as percentage
	trustScoreStr := fmt.Sprintf("%.1f%%", factCheck.TrustScore*100)
	
	// Create embed
	embed := &discordgo.MessageEmbed{
		Title:       "Fact Check: " + item.Title,
		URL:         item.Link,
		Description: factCheck.Claim,
		Color:       color,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Rating",
				Value:  factCheck.Rating,
				Inline: true,
			},
			{
				Name:   "Trust Score",
				Value:  trustScoreStr,
				Inline: true,
			},
			{
				Name:   "Source",
				Value:  source.Name,
				Inline: true,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("Method: %s", factCheck.Method),
		},
		Timestamp: factCheck.CheckedAt.Format(time.RFC3339),
	}
	
	// Add explanation if available
	if factCheck.Explanation != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:  "Explanation",
			Value: factCheck.Explanation,
		})
	}
	
	return embed
}

// performAutoSummarize performs article summarization
func performAutoSummarize(s *discordgo.Session, item *gofeed.Item, source Source) {
	// Only proceed if we have OpenAI API key
	if cfg.OpenAIAPIKey == "" {
		return
	}
	
	// Create article object
	article := &Article{
		Title:     item.Title,
		URL:       item.Link,
		Source:    source.Name,
		Timestamp: *item.PublishedParsed,
	}
	
	// Try to extract more content
	if cfg.EnableImageEmbed {
		extractor := NewArticleExtractor()
		extractedArticle, err := extractor.Extract(item.Link)
		if err == nil {
			article.Content = extractedArticle.Content
		} else {
			// Fallback to description
			if item.Description != "" {
				article.Content = item.Description
			} else if item.Content != "" {
				article.Content = item.Content
			}
		}
	} else {
		// Use description or content
		if item.Description != "" {
			article.Content = item.Description
		} else if item.Content != "" {
			article.Content = item.Content
		}
	}
	
	// Generate summary
	summary, err := SummarizeArticle(article, 500)
	if err != nil {
		Logger().Printf("Auto summarize failed for %s: %v", item.Link, err)
		return
	}
	
	// Only post summary if we have a channel to post to
	if cfg.AuditLogChannelID != "" {
		// Format message
		msg := fmt.Sprintf("**Summary of \"%s\"** from %s\n\n%s\n\n[Read full article](%s)",
			item.Title, source.Name, summary, item.Link)
		
		// Send message
		_, err := s.ChannelMessageSend(cfg.AuditLogChannelID, msg)
		if err != nil {
			Logger().Printf("Failed to send summary: %v", err)
		}
	}
}
