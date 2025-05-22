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
	
	for i, src := range sources {
		if src.Paused {
			continue
		}

		feed, err := parser.ParseURL(src.URL)
		if err != nil {
			Logger().Printf("fetch %s failed: %v", src.Name, err)
			sources[i].LastError = err.Error()
			sources[i].ErrorCount++
			sourcesUpdated = true
			continue
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
					msg += fmt.Sprintf("• [%s](%s) - %s\n", 
						item.Title,
						item.Link, 
						item.PublishedParsed.Format("Jan 02"))
					postCount++
				}
			}
			
			// Send the message
			_, err = s.ChannelMessageSend(channelID, msg)
			if err != nil {
				Logger().Printf("Failed to send message: %v", err)
			}
			
			sources[i].FeedCount += postCount
			sources[i].LastError = ""
			sourcesUpdated = true
		}
	}
	
	// Update the next time in the state
	state.NewsNextTime = time.Now().Add(parseCron(cfg.News15MinCron))
	state.LastInterval = int(parseCron(cfg.News15MinCron).Minutes())
	SaveState(state)
	
	// Save sources if they were updated
	if sourcesUpdated {
		SaveSources(sources)
	}
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
