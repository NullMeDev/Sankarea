// cmd/sankarea/formatter.go
package main

import (
    "fmt"
    "strings"
    "time"

    "github.com/bwmarrin/discordgo"
)

// Formatter handles message formatting and output generation
type Formatter struct {
    maxEmbedLength    int
    maxFieldLength    int
    maxArticlesPerMsg int
}

// NewFormatter creates a new formatter instance
func NewFormatter() *Formatter {
    return &Formatter{
        maxEmbedLength:    6000, // Discord's limit
        maxFieldLength:    1024, // Discord's limit
        maxArticlesPerMsg: 10,   // Reasonable default
    }
}

// FormatNewsDigest creates a formatted news digest for Discord
func (f *Formatter) FormatNewsDigest(articles []*NewsArticle) []*discordgo.MessageSend {
    // Group articles by category
    categories := make(map[string][]*NewsArticle)
    for _, article := range articles {
        categories[article.Category] = append(categories[article.Category], article)
    }

    var messages []*discordgo.MessageSend

    // Create summary embed
    summaryEmbed := &discordgo.MessageEmbed{
        Title:       "📰 News Digest",
        Description: fmt.Sprintf("News summary for %s", time.Now().Format("2006-01-02 15:04 MST")),
        Color:       0x7289DA,
        Fields:      make([]*discordgo.MessageEmbedField, 0),
        Footer: &discordgo.MessageEmbedFooter{
            Text: fmt.Sprintf("Powered by Sankarea v%s", botVersion),
        },
    }

    // Add category summaries
    for category, categoryArticles := range categories {
        summaryEmbed.Fields = append(summaryEmbed.Fields, &discordgo.MessageEmbedField{
            Name:   fmt.Sprintf("%s %s", getCategoryEmoji(category), category),
            Value:  fmt.Sprintf("%d articles", len(categoryArticles)),
            Inline: true,
        })
    }

    messages = append(messages, &discordgo.MessageSend{
        Embeds: []*discordgo.MessageEmbed{summaryEmbed},
    })

    // Create category embeds
    for category, categoryArticles := range categories {
        embeds := f.formatCategoryArticles(category, categoryArticles)
        
        // Split embeds into multiple messages if needed
        currentEmbeds := make([]*discordgo.MessageEmbed, 0)
        currentLength := 0

        for _, embed := range embeds {
            embedLength := f.calculateEmbedLength(embed)
            
            if currentLength+embedLength > f.maxEmbedLength || len(currentEmbeds) >= 10 {
                // Create new message with current embeds
                messages = append(messages, &discordgo.MessageSend{
                    Embeds: currentEmbeds,
                })
                currentEmbeds = make([]*discordgo.MessageEmbed, 0)
                currentLength = 0
            }

            currentEmbeds = append(currentEmbeds, embed)
            currentLength += embedLength
        }

        // Add remaining embeds
        if len(currentEmbeds) > 0 {
            messages = append(messages, &discordgo.MessageSend{
                Embeds: currentEmbeds,
            })
        }
    }

    return messages
}

// formatCategoryArticles creates embeds for articles in a category
func (f *Formatter) formatCategoryArticles(category string, articles []*NewsArticle) []*discordgo.MessageEmbed {
    var embeds []*discordgo.MessageEmbed

    // Calculate how many articles per embed
    articlesPerEmbed := f.maxArticlesPerMsg
    if len(articles) > articlesPerEmbed {
        // Create multiple embeds
        for i := 0; i < len(articles); i += articlesPerEmbed {
            end := i + articlesPerEmbed
            if end > len(articles) {
                end = len(articles)
            }
            embed := f.createCategoryEmbed(category, articles[i:end], i+1)
            embeds = append(embeds, embed)
        }
    } else {
        // Create single embed
        embed := f.createCategoryEmbed(category, articles, 1)
        embeds = append(embeds, embed)
    }

    return embeds
}

// createCategoryEmbed creates an embed for a set of articles
func (f *Formatter) createCategoryEmbed(category string, articles []*NewsArticle, page int) *discordgo.MessageEmbed {
    embed := &discordgo.MessageEmbed{
        Title: fmt.Sprintf("%s %s News", getCategoryEmoji(category), category),
        Color: getCategoryColor(category),
        Fields: make([]*discordgo.MessageEmbedField, 0),
    }

    for _, article := range articles {
        // Create article field
        field := &discordgo.MessageEmbedField{
            Name: f.truncateString(article.Title, 256),
            Value: f.formatArticleField(article),
            Inline: false,
        }
        embed.Fields = append(embed.Fields, field)
    }

    // Add footer if multiple pages
    if page > 1 {
        embed.Footer = &discordgo.MessageEmbedFooter{
            Text: fmt.Sprintf("Page %d", page),
        }
    }

    return embed
}

// formatArticleField formats the content of an article field
func (f *Formatter) formatArticleField(article *NewsArticle) string {
    var parts []string

    // Add source and timestamp
    parts = append(parts, fmt.Sprintf("Source: %s | %s",
        article.Source,
        article.PublishedAt.Format("2006-01-02 15:04 MST")))

    // Add URL
    parts = append(parts, article.URL)

    // Add reliability badge if available
    if article.FactCheckResult != nil {
        parts = append(parts, getReliabilityBadge(article.FactCheckResult))
    }

    // Join with newlines and truncate if needed
    return f.truncateString(strings.Join(parts, "\n"), f.maxFieldLength)
}

// Helper functions

func (f *Formatter) calculateEmbedLength(embed *discordgo.MessageEmbed) int {
    length := len(embed.Title) + len(embed.Description)
    
    for _, field := range embed.Fields {
        length += len(field.Name) + len(field.Value)
    }

    if embed.Footer != nil {
        length += len(embed.Footer.Text)
    }

    return length
}

func (f *Formatter) truncateString(s string, maxLen int) string {
    if len(s) <= maxLen {
        return s
    }
    return s[:maxLen-3] + "..."
}

func getReliabilityBadge(result *FactCheckResult) string {
    var emoji string
    switch result.ReliabilityTier {
    case "High":
        emoji = "🟢"
    case "Medium":
        emoji = "🟡"
    case "Low":
        emoji = "🔴"
    default:
        emoji = "⚫"
    }
    return fmt.Sprintf("%s Reliability: %s (%.1f/1.0)", emoji, result.ReliabilityTier, result.Score)
}

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
