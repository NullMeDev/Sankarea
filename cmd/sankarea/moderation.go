package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/sashabaranov/go-openai"
)

// ContentSeverity levels for content moderation
const (
	SeverityNone     = 0
	SeverityLow      = 1
	SeverityMedium   = 2
	SeverityHigh     = 3
)

// ContentModeration represents OpenAI moderation results
type ContentModeration struct {
	Flagged     bool
	Categories  map[string]bool
	Severity    int
	Explanation string
}

// ModerateContent checks content for policy violations using OpenAI's moderation API
func ModerateContent(content string) (*ContentModeration, error) {
	if !cfg.EnableContentFiltering || cfg.OpenAIAPIKey == "" {
		return nil, fmt.Errorf("Content moderation not configured")
	}

	client := openai.NewClient(cfg.OpenAIAPIKey)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	// Limit content length
	if len(content) > 2000 {
		content = content[:2000]
	}

	// Call moderation API
	response, err := client.Moderations(ctx, openai.ModerationRequest{
		Input: content,
	})
	if err != nil {
		return nil, fmt.Errorf("OpenAI moderation API error: %v", err)
	}

	if len(response.Results) == 0 {
		return nil, fmt.Errorf("No moderation results returned")
	}

	result := response.Results[0]

	// Determine severity based on scores
	severity := SeverityNone
	var flaggedCategories []string
	maxScore := 0.0

	// Check each category
	if result.Categories.Harassment && result.CategoryScores.Harassment > 0.5 {
		flaggedCategories = append(flaggedCategories, "harassment")
		severity = max(severity, SeverityMedium)
		maxScore = max(maxScore, result.CategoryScores.Harassment)
	}
	if result.Categories.HateThreatening && result.CategoryScores.HateThreatening > 0.5 {
		flaggedCategories = append(flaggedCategories, "harmful content")
		severity = max(severity, SeverityMedium)
		maxScore = max(maxScore, result.CategoryScores.HateThreatening)
	}
	if result.Categories.Hate && result.CategoryScores.Hate > 0.5 {
		flaggedCategories = append(flaggedCategories, "hate")
		severity = max(severity, SeverityHigh)
		maxScore = max(maxScore, result.CategoryScores.Hate)
	}
	if result.Categories.SelfHarm && result.CategoryScores.SelfHarm > 0.5 {
		flaggedCategories = append(flaggedCategories, "self-harm")
		severity = max(severity, SeverityHigh)
		maxScore = max(maxScore, result.CategoryScores.SelfHarm)
	}
	if result.Categories.Sexual && result.CategoryScores.Sexual > 0.5 {
		flaggedCategories = append(flaggedCategories, "sexual content")
		severity = max(severity, SeverityMedium)
		maxScore = max(maxScore, result.CategoryScores.Sexual)
	}
	if result.Categories.Violence && result.CategoryScores.Violence > 0.5 {
		flaggedCategories = append(flaggedCategories, "violence")
		severity = max(severity, SeverityMedium)
		maxScore = max(maxScore, result.CategoryScores.Violence)
	}

	// Build explanation
	explanation := ""
	if len(flaggedCategories) > 0 {
		explanation = fmt.Sprintf("Content flagged for %s with %0.1f%% confidence", 
			strings.Join(flaggedCategories, ", "), maxScore*100)
	}

	return &ContentModeration{
		Flagged:     result.Flagged,
		Categories:  map[string]bool{
			"harassment":  result.Categories.Harassment,
			"harmful":     result.Categories.HateThreatening,
			"hate":        result.Categories.Hate,
			"self_harm":   result.Categories.SelfHarm,
			"sexual":      result.Categories.Sexual,
			"violence":    result.Categories.Violence,
		},
		Severity:    severity,
		Explanation: explanation,
	}, nil
}

// HandleModeratedContent takes action based on moderation result
func HandleModeratedContent(s *discordgo.Session, channelID, messageID, content string, result *ContentModeration) {
	if !result.Flagged || result.Severity < SeverityMedium {
		return // No action needed
	}

	// Log the moderation event
	Logger().Printf("Content moderation alert: %s (severity %d)", result.Explanation, result.Severity)

	// For higher severity, delete the message
	if result.Severity >= SeverityHigh {
		err := s.ChannelMessageDelete(channelID, messageID)
		if err != nil {
			Logger().Printf("Failed to delete flagged message: %v", err)
		} else {
			Logger().Printf("Deleted message with severity %d from channel %s", result.Severity, channelID)
		}
	}

	// Send notification to audit log
	if cfg.AuditLogChannelID != "" {
		embed := &discordgo.MessageEmbed{
			Title:       "Content Moderation Alert",
			Description: result.Explanation,
			Color:       0xFF0000, // Red
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:   "Severity",
					Value:  fmt.Sprintf("%d/3", result.Severity),
					Inline: true,
				},
				{
					Name:   "Channel",
					Value:  fmt.Sprintf("<#%s>", channelID),
					Inline: true,
				},
			},
			Timestamp: time.Now().Format(time.RFC3339),
		}
		
		if result.Severity < SeverityHigh {
			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
				Name:   "Content",
				Value:  truncateString(content, 200),
				Inline: false,
			})
		}
		
		_, err := s.ChannelMessageSendEmbed(cfg.AuditLogChannelID, embed)
		if err != nil {
			Logger().Printf("Failed to send moderation alert: %v", err)
		}
	}
}

// Helper function to limit string length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// max returns the maximum of two float64s
func maxFloat64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
