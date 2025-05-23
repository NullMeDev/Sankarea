// cmd/sankarea/digest.go
package main

import (
    "fmt"
    "sort"
    "strings"
    "time"

    "github.com/bwmarrin/discordgo"
)

// DigestResult represents a formatted news digest
type DigestResult struct {
    Embeds     []*discordgo.MessageEmbed
    TotalNews  int
    Categories map[string]int
}

// generateDigest creates a news digest for the specified time range
func generateDigest(startTime, endTime time.Time) (*DigestResult, error) {
    // Load state for recent articles
    state, err := LoadState()
    if err != nil {
        return nil, fmt.Errorf("failed to load state: %v", err)
    }

    // Filter articles within time range
    var articles []*NewsArticle
    for _, article := range state.RecentArticles {
        if article.PublishedAt.After(startTime) && article.PublishedAt.Before(endTime) {
            articles = append(articles, article)
        }
    }

    // Sort articles by category and then by publish date
    sort.Slice(articles, func(i, j int) bool {
        if articles[i].Category == articles[j].Category {
            return articles[i].PublishedAt.After(articles[j].PublishedAt)
        }
        return articles[i].Category < articles[j].Category
    })

    // Group articles by category
    categoryArticles := make(map[string][]*NewsArticle)
    categoryCount := make(map[string]int)
    for _, article := range articles {
        categoryArticles[article.Category] = append(categoryArticles[article.Category], article)
        categoryCount[article.Category]++
    }

    // Create digest embeds
    var embeds []*discordgo.MessageEmbed

    // Summary embed
    summaryEmbed := &discordgo.MessageEmbed{
        Title: "📰 News Digest Summary",
        Description: fmt.Sprintf("News from %s to %s",
            startTime.Format("2006-01-02 15:04 MST"),
            endTime.Format("2006-01-02 15:04 MST")),
        Fields: make([]*discordgo.MessageEmbedField, 0),
        Color:  0x7289DA,
    }

    // Add category summaries
    for category, count := range categoryCount {
        summaryEmbed.Fields = append(summaryEmbed.Fields, &discordgo.MessageEmbedField{
            Name:   getCategoryEmoji(category) + " " + category,
            Value:  fmt.Sprintf("%d articles", count),
            Inline: true,
        })
    }

    embeds = append(embeds, summaryEmbed)

    // Category embeds
    for category, articles := range categoryArticles {
        // Skip categories with no articles
        if len(articles) == 0 {
            continue
        }

        // Create category embed
        categoryEmbed := &discordgo.MessageEmbed{
            Title: fmt.Sprintf("%s %s News", getCategoryEmoji(category), category),
            Color: getCategoryColor(category),
            Fields: make([]*discordgo.MessageEmbedField, 0),
        }

        // Add top articles
        maxArticles := 10 // Maximum articles per category
        for i, article := range articles {
            if i >= maxArticles {
                break
            }

            // Create article field
            field := &discordgo.MessageEmbedField{
                Name: truncateString(article.Title, 256),
                Value: fmt.Sprintf("Source: %s\n%s\n%s",
                    article.Source,
                    article.URL,
                    getReliabilityBadge(article)),
                Inline: false,
            }
            categoryEmbed.Fields = append(categoryEmbed.Fields, field)
        }

        // Add overflow message if needed
        if len(articles) > maxArticles {
            categoryEmbed.Footer = &discordgo.MessageEmbedFooter{
                Text: fmt.Sprintf("And %d more articles...", len(articles)-maxArticles),
            }
        }

        embeds = append(embeds, categoryEmbed)
    }

    return &DigestResult{
        Embeds:     embeds,
        TotalNews:  len(articles),
        Categories: categoryCount,
    }, nil
}

// getReliabilityBadge returns a formatted reliability indicator
func getReliabilityBadge(article *NewsArticle) string {
    if article.FactCheckResult == nil {
        return "🔄 Fact check pending"
    }

    var badge string
    switch article.FactCheckResult.ReliabilityTier {
    case "High":
        badge = "🟢 High reliability"
    case "Medium":
        badge = "🟡 Medium reliability"
    case "Low":
        badge = "🔴 Low reliability"
    default:
        badge = "⚫ Unknown reliability"
    }

    return fmt.Sprintf("%s (%.1f/1.0)", badge, article.FactCheckResult.Score)
}

// getCategoryEmoji returns an emoji for the given category
func getCategoryEmoji(category string) string {
    switch category {
    case CategoryTechnology:
        return "💻"
    case CategoryBusiness:
        return "💼"
    case CategoryScience:
        return "🔬"
    case CategoryHealth:
        return "🏥"
    case CategoryPolitics:
        return "🏛️"
    case CategorySports:
        return "⚽"
    case CategoryWorld:
        return "🌍"
    default:
        return "📰"
    }
}

// getCategoryColor returns a color for the given category
func getCategoryColor(category string) int {
    switch category {
    case CategoryTechnology:
        return 0x7289DA // Discord Blue
    case CategoryBusiness:
        return 0x43B581 // Green
    case CategoryScience:
        return 0xFAA61A // Orange
    case CategoryHealth:
        return 0xF04747 // Red
    case CategoryPolitics:
        return 0x747F8D // Gray
    case CategorySports:
        return 0x2ECC71 // Emerald
    case CategoryWorld:
        return 0x99AAB5 // Light Gray
    default:
        return 0x000000 // Black
    }
}

// truncateString truncates a string to the specified length
func truncateString(s string, maxLen int) string {
    if len(s) <= maxLen {
        return s
    }
    return s[:maxLen-3] + "..."
}
