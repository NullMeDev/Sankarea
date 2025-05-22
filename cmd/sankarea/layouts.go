package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mmcdole/gofeed"
)

// PostLayout defines different layouts for news posts
type PostLayout int

const (
	LayoutMinimal PostLayout = iota
	LayoutStandard
	LayoutDetailed
	LayoutEmbed
)

// FormatNewsPost formats a news item according to the selected layout
func FormatNewsPost(s *discordgo.Session, channelID string, source Source, feed *gofeed.Feed, items []*gofeed.Item, layout PostLayout) error {
	switch layout {
	case LayoutMinimal:
		return formatMinimalNewsPost(s, channelID, source, feed, items)
	case LayoutStandard:
		return formatStandardNewsPost(s, channelID, source, feed, items)
	case LayoutEmbed:
		return formatEmbedNewsPost(s, channelID, source, feed, items)
	case LayoutDetailed:
		return formatDetailedNewsPost(s, channelID, source, feed, items)
	default:
		return formatMinimalNewsPost(s, channelID, source, feed, items)
	}
}

// formatMinimalNewsPost creates a clean, minimal post format
func formatMinimalNewsPost(s *discordgo.Session, channelID string, source Source, feed *gofeed.Feed, items []*gofeed.Item) error {
	if len(items) == 0 {
		return nil
	}

	// Create a simple header
	biasIndicator := ""
	if source.Bias != "" {
		biasIndicator = fmt.Sprintf(" • %s", source.Bias)
	}
	
	header := fmt.Sprintf("**%s**%s", source.Name, biasIndicator)
	
	// Format each item in a minimal way
	var lines []string
	for _, item := range items {
		if item.PublishedParsed == nil {
			item.PublishedParsed = &time.Time{}
		}
		
		// Format: • Title (time)
		line := fmt.Sprintf("• [%s](%s) `%s`", 
			cleanTitle(item.Title),
			item.Link,
			item.PublishedParsed.Format("15:04"))
			
		lines = append(lines, line)
	}
	
	// Combine and send the message
	message := header + "\n" + strings.Join(lines, "\n")
	_, err := s.ChannelMessageSend(channelID, message)
	return err
}

// formatStandardNewsPost creates a standard format post with source info and items
func formatStandardNewsPost(s *discordgo.Session, channelID string, source Source, feed *gofeed.Feed, items []*gofeed.Item) error {
	if len(items) == 0 {
		return nil
	}
	
	biasIndicator := ""
	if source.Bias != "" {
		biasIndicator = fmt.Sprintf(" (%s)", source.Bias)
	}
	
	header := fmt.Sprintf("🔗 **%s**%s: %s", source.Name, biasIndicator, feed.Title)
	
	var lines []string
	for _, item := range items {
		if item.PublishedParsed == nil {
			item.PublishedParsed = &time.Time{}
		}
		
		line := fmt.Sprintf("• [%s](%s) - %s", 
			cleanTitle(item.Title),
			item.Link,
			item.PublishedParsed.Format("Jan 02"))
			
		lines = append(lines, line)
	}
	
	message := header + "\n" + strings.Join(lines, "\n")
	_, err := s.ChannelMessageSend(channelID, message)
	return err
}

// formatEmbedNewsPost creates an embed for the news post
func formatEmbedNewsPost(s *discordgo.Session, channelID string, source Source, feed *gofeed.Feed, items []*gofeed.Item) error {
	if len(items) == 0 {
		return nil
	}
	
	// Determine color based on bias
	color := 0x0099FF // Default blue
	switch strings.ToLower(source.Bias) {
	case "left":
		color = 0x0000FF // Blue
	case "left-center":
		color = 0x00AAFF // Light blue
	case "center":
		color = 0x808080 // Gray
	case "right-center":
		color = 0xFFAA00 // Light red
	case "right":
		color = 0xFF0000 // Red
	}
	
	// Create embed
	embed := &discordgo.MessageEmbed{
		Title:       feed.Title,
		Description: fmt.Sprintf("Latest news from %s (%s)", source.Name, source.Bias),
		Color:       color,
		Timestamp:   time.Now().Format(time.RFC3339),
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Powered by Sankarea News Bot",
		},
	}
	
	// Add fields for each item
	for _, item := range items {
		if item.PublishedParsed == nil {
			item.PublishedParsed = &time.Time{}
		}
		
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name: cleanTitle(item.Title),
			Value: fmt.Sprintf("[Read more](%s) • %s", 
				item.Link, 
				item.PublishedParsed.Format("Jan 02, 15:04")),
			Inline: false,
		})
	}
	
	// Add source image if available
	if feed.Image != nil && feed.Image.URL != "" {
		embed.Thumbnail = &discordgo.MessageEmbedThumbnail{
			URL: feed.Image.URL,
		}
	}
	
	// Send the embed
	_, err := s.ChannelMessageSendEmbed(channelID, embed)
	return err
}

// formatDetailedNewsPost creates a detailed post with article previews
func formatDetailedNewsPost(s *discordgo.Session, channelID string, source Source, feed *gofeed.Feed, items []*gofeed.Item) error {
	if len(items) == 0 {
		return nil
	}
	
	// Create header embed
	headerEmbed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("%s: %s", source.Name, feed.Title),
		Description: feed.Description,
		URL:         feed.Link,
		Color:       0x0099FF,
		Timestamp:   time.Now().Format(time.RFC3339),
	}
	
	if feed.Image != nil && feed.Image.URL != "" {
		headerEmbed.Thumbnail = &discordgo.MessageEmbedThumbnail{
			URL: feed.Image.URL,
		}
	}
	
	_, err := s.ChannelMessageSendEmbed(channelID, headerEmbed)
	if err != nil {
		return err
	}
	
	// Send individual embeds for each item
	for _, item := range items {
		if item.PublishedParsed == nil {
			item.PublishedParsed = &time.Time{}
		}
		
		// Create item embed
		itemEmbed := &discordgo.MessageEmbed{
			Title:       cleanTitle(item.Title),
			URL:         item.Link,
			Description: getItemDescription(item),
			Color:       0x0099FF,
			Footer: &discordgo.MessageEmbedFooter{
				Text: fmt.Sprintf("%s • %s", 
					source.Name, 
					item.PublishedParsed.Format("Jan 02, 15:04")),
			},
		}
		
		// Add image if available
		image := getItemImage(item)
		if image != "" {
			itemEmbed.Image = &discordgo.MessageEmbedImage{
				URL: image,
			}
		}
		
		_, err := s.ChannelMessageSendEmbed(channelID, itemEmbed)
		if err != nil {
			Logger().Printf("Failed to send item embed: %v", err)
			continue
		}
		
		// Add a small delay to avoid rate limiting
		time.Sleep(500 * time.Millisecond)
	}
	
	return nil
}

// Helper function to clean up titles
func cleanTitle(title string) string {
	// Remove excess whitespace
	title = strings.TrimSpace(title)
	
	// Truncate if too long
	if len(title) > 100 {
		title = title[:97] + "..."
	}
	
	return title
}

// Helper function to get item description
func getItemDescription(item *gofeed.Item) string {
	// Try to get a good description
	desc := item.Description
	
	// If description is empty or too long, try content
	if desc == "" || len(desc) > 500 {
		if item.Content != "" {
			desc = item.Content
		}
	}
	
	// Clean up HTML
	desc = strings.ReplaceAll(desc, "<p>", "")
	desc = strings.ReplaceAll(desc, "</p>", "\n\n")
	desc = strings.ReplaceAll(desc, "<br>", "\n")
	desc = strings.ReplaceAll(desc, "<br/>", "\n")
	desc = strings.ReplaceAll(desc, "<br />", "\n")
	
	// Remove all other HTML tags
	for strings.Contains(desc, "<") && strings.Contains(desc, ">") {
		startIdx := strings.Index(desc, "<")
		endIdx := strings.Index(desc, ">")
		if startIdx < endIdx {
			desc = desc[:startIdx] + desc[endIdx+1:]
		} else {
			break
		}
	}
	
	// Truncate if too long
	if len(desc) > 300 {
		desc = desc[:297] + "..."
	}
	
	return strings.TrimSpace(desc)
}

// Helper function to get image from item
func getItemImage(item *gofeed.Item) string {
	// Check for media content
	if len(item.Enclosures) > 0 {
		for _, enclosure := range item.Enclosures {
			if strings.HasPrefix(enclosure.Type, "image/") {
				return enclosure.URL
			}
		}
	}
	
	// Try to extract image from content
	if item.Content != "" {
		imgStart := strings.Index(strings.ToLower(item.Content), "<img")
		if imgStart >= 0 {
			srcStart := strings.Index(strings.ToLower(item.Content[imgStart:]), "src=\"")
			if srcStart >= 0 {
				srcStart += imgStart + 5 // 5 is length of 'src="'
				srcEnd := strings.Index(item.Content[srcStart:], "\"")
				if srcEnd > 0 {
					return item.Content[srcStart : srcStart+srcEnd]
				}
			}
		}
	}
	
	return ""
}
