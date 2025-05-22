package main

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/mmcdole/gofeed"
)

// ChannelConfiguration defines news delivery settings for channels
type ChannelConfiguration struct {
	ChannelID    string
	Categories   []string // Categories to include
	ExcludeSources []string // Sources to exclude
	IncludeSources []string // Sources to include (takes precedence over exclude)
	MinTrustScore float64 // Minimum trust score for articles
	SentimentFilter string // "positive", "negative", "neutral", "all"
	MaxArticlesPerUpdate int // Maximum articles per update
	UseSummaries bool // Whether to use summaries instead of full content
	UseFactChecking bool // Whether to add fact checking to posts
	FormatStyle string // "compact", "detailed", "embed"
}

// NewsDeliverySystem manages delivering news to Discord channels
type NewsDeliverySystem struct {
	session         *discordgo.Session
	defaultChannel  string
	channelConfigs  map[string]ChannelConfiguration
}

// NewNewsDeliverySystem creates a new news delivery system
func NewNewsDeliverySystem(s *discordgo.Session, defaultChannel string) *NewsDeliverySystem {
	return &NewsDeliverySystem{
		session:        s,
		defaultChannel: defaultChannel,
		channelConfigs: make(map[string]ChannelConfiguration),
	}
}

// AddChannelConfig adds or updates channel configuration
func (nds *NewsDeliverySystem) AddChannelConfig(config ChannelConfiguration) {
	nds.channelConfigs[config.ChannelID] = config
}

// RemoveChannelConfig removes a channel configuration
func (nds *NewsDeliverySystem) RemoveChannelConfig(channelID string) {
	delete(nds.channelConfigs, channelID)
}

// GetTargetChannels determines which channels should receive an article
func (nds *NewsDeliverySystem) GetTargetChannels(sourceName, category string, trustScore float64, sentiment string) []string {
	// First check source channel override
	sources, _ := LoadSources()
	for _, src := range sources {
		if strings.EqualFold(src.Name, sourceName) && src.ChannelOverride != "" {
			return []string{src.ChannelOverride}
		}
	}

	// Then check categories and other rules
	channels := []string{}
	
	// Add default channel
	channels = append(channels, nds.defaultChannel)
	
	// Check each configured channel
	for channelID, config := range nds.channelConfigs {
		// Skip if this is the default channel
		if channelID == nds.defaultChannel {
			continue
		}
		
		// Check if source is explicitly included
		sourceIncluded := false
		for _, includedSource := range config.IncludeSources {
			if strings.EqualFold(includedSource, sourceName) {
				sourceIncluded = true
				break
			}
		}
		
		// Skip if source is excluded and not explicitly included
		if !sourceIncluded {
			for _, excludedSource := range config.ExcludeSources {
				if strings.EqualFold(excludedSource, sourceName) {
					continue
				}
			}
		}
		
		// Check category match
		categoryMatch := len(config.Categories) == 0 // Empty means all categories
		for _, allowedCategory := range config.Categories {
			if strings.EqualFold(allowedCategory, category) {
				categoryMatch = true
				break
			}
		}
		
		// Check trust score
		if trustScore < config.MinTrustScore {
			continue
		}
		
		// Check sentiment filter
		if config.SentimentFilter != "all" && config.SentimentFilter != "" && 
		   !strings.EqualFold(config.SentimentFilter, sentiment) {
			continue
		}
		
		// If all checks pass, add this channel to target list
		if categoryMatch || sourceIncluded {
			channels = append(channels, channelID)
		}
	}
	
	return channels
}

// DeliverNewsItem delivers a news item to appropriate channels
func (nds *NewsDeliverySystem) DeliverNewsItem(
	item *gofeed.Item, 
	sourceName string, 
	source *Source,
	summary string, 
	factCheck string,
	sentiment *SentimentAnalysis,
) error {
	category := source.Category
	if category == "" && sentiment != nil && len(sentiment.Topics) > 0 {
		category = GetArticleCategoryRecommendation(sentiment)
	}
	
	// Determine sentiment string
	sentimentStr := "neutral"
	if sentiment != nil {
		sentimentStr = sentiment.Sentiment
	}
	
	// Determine channels to post to
	var trustScore float64 = 0.5 // Default neutral score
	if source != nil {
		trustScore = source.TrustScore
	}
	
	channels := nds.GetTargetChannels(sourceName, category, trustScore, sentimentStr)
	
	// Send to each channel with appropriate formatting
	for _, channelID := range channels {
		var messageContent string
		var embeds []*discordgo.MessageEmbed
		
		// Get channel config if exists
		config, hasConfig := nds.channelConfigs[channelID]
		if !hasConfig {
			// Use default simple format
			messageContent = formatNewsSimple(item, sourceName, category, false, false)
		} else {
			// Use channel-specific formatting
			switch config.FormatStyle {
			case "compact":
				messageContent = formatNewsSimple(item, sourceName, category, false, false)
			case "detailed":
				messageContent = formatNewsDetailed(item, sourceName, category, summary, factCheck, sentiment, config.UseFactChecking, config.UseSummaries)
			case "embed":
				embeds = formatNewsEmbed(item, sourceName, category, summary, factCheck, sentiment, config.UseFactChecking, config.UseSummaries)
			default:
				messageContent = formatNewsSimple(item, sourceName, category, config.UseFactChecking, config.UseSummaries)
			}
		}
		
		// Send the message to this channel
		if len(embeds) > 0 {
			_, err := nds.session.ChannelMessageSendEmbeds(channelID, embeds)
			if err != nil {
				Logger().Printf("Error sending news to channel %s: %v", channelID, err)
			}
		} else if messageContent != "" {
			_, err := nds.session.ChannelMessageSend(channelID, messageContent)
			if err != nil {
				Logger().Printf("Error sending news to channel %s: %v", channelID, err)
			}
		}
	}
	
	return nil
}

// formatNewsSimple formats a news item in a simple format
func formatNewsSimple(item *gofeed.Item, sourceName, category string, includeFactCheck, includeSummary bool) string {
	var sb strings.Builder
	
	// Source and title
	sb.WriteString(fmt.Sprintf("🔍 **%s** ", sourceName))
	if category != "" {
		sb.WriteString(fmt.Sprintf("[%s] ", category))
	}
	
	if item.Title != "" {
		sb.WriteString(fmt.Sprintf("| %s\n", item.Title))
	}
	
	// Link
	if item.Link != "" {
		sb.WriteString(fmt.Sprintf("🔗 %s\n", item.Link))
	}
	
	// Publication date
	if item.PublishedParsed != nil {
		sb.WriteString(fmt.Sprintf("📅 Published <t:%d:R>\n", item.PublishedParsed.Unix()))
	}
	
	return sb.String()
}

// formatNewsDetailed formats a news item in a detailed format
func formatNewsDetailed(
	item *gofeed.Item, 
	sourceName, 
	category string, 
	summary string, 
	factCheck string,
	sentiment *SentimentAnalysis,
	includeFactCheck bool, 
	includeSummary bool,
) string {
	var sb strings.Builder
	
	// Header with source and category
	sb.WriteString(fmt.Sprintf("📰 **%s", sourceName))
	if category != "" {
		sb.WriteString(fmt.Sprintf(" [%s]", category))
	}
	sb.WriteString("**\n\n")
	
	// Title
	if item.Title != "" {
		sb.WriteString(fmt.Sprintf("## %s\n\n", item.Title))
	}
	
	// Link
	if item.Link != "" {
		sb.WriteString(fmt.Sprintf("🔗 %s\n\n", item.Link))
	}
	
	// Summary
	if includeSummary && summary != "" {
		sb.WriteString("**Summary:**\n")
		sb.WriteString(summary)
		sb.WriteString("\n\n")
	}
	
	// Sentiment
	if sentiment != nil {
		sentimentEmoji := "🟡" // neutral
		if sentiment.Sentiment == "positive" {
			sentimentEmoji = "🟢"
		} else if sentiment.Sentiment == "negative" {
			sentimentEmoji = "🔴"
		}
		
		sb.WriteString(fmt.Sprintf("%s **Sentiment:** %s\n", sentimentEmoji, sentiment.Sentiment))
		
		if len(sentiment.Topics) > 0 {
			sb.WriteString(fmt.Sprintf("🏷️ **Topics:** %s\n", strings.Join(sentiment.Topics, ", ")))
		}
	}
	
	// Fact check
	if includeFactCheck && factCheck != "" {
		sb.WriteString("\n**Fact Check:**\n")
		sb.WriteString(factCheck)
		sb.WriteString("\n")
	}
	
	// Publication date
	if item.PublishedParsed != nil {
		sb.WriteString(fmt.Sprintf("\n📅 Published <t:%d:R>\n", item.PublishedParsed.Unix()))
	}
	
	return sb.String()
}

// formatNewsEmbed formats a news item as a Discord embed
func formatNewsEmbed(
	item *gofeed.Item, 
	sourceName, 
	category string, 
	summary string, 
	factCheck string,
	sentiment *SentimentAnalysis,
	includeFactCheck bool, 
	includeSummary bool,
) []*discordgo.MessageEmbed {
	// Create the main embed
	embed := &discordgo.MessageEmbed{
		Title:       item.Title,
		URL:         item.Link,
		Description: summary,
		Color:       0x4B9CD3, // Blue
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("%s • %s", sourceName, category),
		},
		Fields: []*discordgo.MessageEmbedField{},
	}
	
	// Add timestamp
	if item.PublishedParsed != nil {
		embed.Timestamp = item.PublishedParsed.Format(time.RFC3339)
	}
	
	// Add thumbnail if available
	if item.Image != nil && item.Image.URL != "" {
		embed.Thumbnail = &discordgo.MessageEmbedThumbnail{
			URL: item.Image.URL,
		}
	}
	
	// Add sentiment analysis
	if sentiment != nil {
		// Determine color based on sentiment
		if sentiment.Sentiment == "positive" {
			embed.Color = 0x43B581 // Green
		} else if sentiment.Sentiment == "negative" {
			embed.Color = 0xF04747 // Red
		}
		
		// Add topics as a field
		if len(sentiment.Topics) > 0 {
			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
				Name:   "Topics",
				Value:  strings.Join(sentiment.Topics, ", "),
				Inline: true,
			})
		}
		
		// Add sentiment field
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "Sentiment",
			Value:  fmt.Sprintf("%s (%.1f/1.0)", strings.Title(sentiment.Sentiment), sentiment.Score),
			Inline: true,
		})
	}
	
	// Add fact check if requested
	if includeFactCheck && factCheck != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "Fact Check",
			Value:  factCheck,
			Inline: false,
		})
	}
	
	return []*discordgo.MessageEmbed{embed}
}
