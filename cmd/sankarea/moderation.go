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
	if result.Categories.HarmfulContent && result.CategoryScores.HarmfulContent > 0.5 {
		flaggedCategories = append(flaggedCategories, "harmful content")
		severity = max(severity, SeverityMedium)
		maxScore = max(maxScore, result.CategoryScores.HarmfulContent)
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
		explanation = fmt.Sprintf("Content flagged for: %s (confidence: %.1f%%)", 
			strings.Join(flaggedCategories, ", "), maxScore*100)
	}

	// Create moderation result
	modResult := &ContentModeration{
		Flagged:     result.Flagged,
		Categories:  make(map[string]bool),
		Severity:    severity,
		Explanation: explanation,
	}

	// Copy categories
	modResult.Categories["harassment"] = result.Categories.Harassment
	modResult.Categories["harmful"] = result.Categories.HarmfulContent
	modResult.Categories["hate"] = result.Categories.Hate
	modResult.Categories["self-harm"] = result.Categories.SelfHarm
	modResult.Categories["sexual"] = result.Categories.Sexual
	modResult.Categories["violence"] = result.Categories.Violence

	return modResult, nil
}

// ShouldAllowContent checks if content should be allowed based on moderation
func ShouldAllowContent(mod *ContentModeration) bool {
	if mod == nil {
		return true
	}

	// Block high severity content
	if mod.Severity >= SeverityHigh {
		return false
	}

	// Allow everything else
	return true
}

// FilterNewsContent filters news content based on moderation
func FilterNewsContent(title, content string) (string, string, bool, string) {
	combined := title + "\n\n" + content
	
	// Check if content filtering is enabled
	if !cfg.EnableContentFiltering {
		return title, content, true, ""
	}
	
	// Perform moderation check
	mod, err := ModerateContent(combined)
	if err != nil {
		Logger().Printf("Moderation error: %v", err)
		return title, content, true, ""  // Allow on error
	}
	
	// Decide whether to allow content
	allowed := ShouldAllowContent(mod)
	
	// If rejected, return reason
	if !allowed {
		return "", "", false, mod.Explanation
	}
	
	// If medium severity, add warning
	if mod.Severity == SeverityMedium {
		warningMsg := fmt.Sprintf("⚠️ Content Warning: This article may contain sensitive material (%s)", mod.Explanation)
		content = warningMsg + "\n\n" + content
	}
	
	return title, content, true, ""
}

// SendContentWarning sends a warning about filtered content
func SendContentWarning(s *discordgo.Session, channelID, sourceName, reason string) {
	// Build warning message
	msg := fmt.Sprintf("⚠️ **Content Warning**: Article from **%s** was filtered due to %s", 
		sourceName, reason)
	
	// Log the warning
	Logger().Printf("Content filtered: %s - %s", sourceName, reason)
	
	// Send as message
	s.ChannelMessageSend(channelID, msg)
}
