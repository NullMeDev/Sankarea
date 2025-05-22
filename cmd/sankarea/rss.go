package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mmcdole/gofeed"
	"github.com/robfig/cron/v3"
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

// postNewsDigest aggregates and posts a daily digest
func postNewsDigest(s *discordgo.Session, channelID string, sources []Source) {
	state, err := LoadState()
	if err != nil {
		Logger().Printf("Cannot load state: %v", err)
		return
	}

	if state.Paused {
		Logger().Println("News digest paused by system state")
		return
	}
	
	// Post header message
	header := fmt.Sprintf("📰 **Daily News Digest** (%s)", time.Now().Format("2006-01-02"))
	_, err = s.ChannelMessageSend(channelID, header)
	if err != nil {
		Logger().Printf("Failed to send digest header: %v", err)
	}
	
	// Collect all sources
	activeSources := 0
	for _, src := range sources {
		if !src.Paused {
			activeSources++
		}
	}
	
	summary := fmt.Sprintf("Monitoring %d active sources", activeSources)
	_, err = s.ChannelMessageSend(channelID, summary)
	if err != nil {
		Logger().Printf("Failed to send digest summary: %v", err)
	}
	
	// Post detailed news
	fetchAndPostNews(s, channelID, sources)
	
	// Update state
	state.LastDigest = time.Now()
	SaveState(state)
}

// parseCron turns a "*/N * * * *" spec into minutes
func parseCron(cronSpec string) time.Duration {
	// look for "/N"
	parts := strings.Split(cronSpec, "/")
	if len(parts) >= 2 {
		nPart := strings.Fields(parts[1])[0]
		if n, err := strconv.Atoi(nPart); err == nil {
			return time.Duration(n) * time.Minute
		}
	}
	// fallback
	return 15 * time.Minute
}

// UpdateCronJob installs or updates the 15-min job
func UpdateCronJob(c *cron.Cron, entryID *cron.EntryID, minutes int, dg *discordgo.Session, channelID string, sources []Source) {
	if *entryID > 0 {
		c.Remove(*entryID)
	}
	spec := fmt.Sprintf("@every %dm", minutes)
	id, err := c.AddFunc(spec, func() {
		fetchAndPostNews(dg, channelID, sources)
	})
	if err != nil {
		Logger().Printf("cron [%s] failed: %v", spec, err)
	} else {
		*entryID = id
		Logger().Printf("Scheduled news updates every %d minutes", minutes)
	}
}
