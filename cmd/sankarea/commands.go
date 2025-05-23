// cmd/sankarea/commands.go
package main

import (
    "fmt"
    "strings"
    "time"

    "github.com/bwmarrin/discordgo"
)

var (
    commands = []*discordgo.ApplicationCommand{
        {
            Name:        "news",
            Description: "Get the latest news",
            Options: []*discordgo.ApplicationCommandOption{
                {
                    Type:        discordgo.ApplicationCommandOptionString,
                    Name:        "category",
                    Description: "News category to filter by",
                    Required:    false,
                    Choices: []*discordgo.ApplicationCommandOptionChoice{
                        {Name: "Technology", Value: CategoryTechnology},
                        {Name: "Business", Value: CategoryBusiness},
                        {Name: "Science", Value: CategoryScience},
                        {Name: "Health", Value: CategoryHealth},
                        {Name: "Politics", Value: CategoryPolitics},
                        {Name: "Sports", Value: CategorySports},
                        {Name: "World", Value: CategoryWorld},
                    },
                },
            },
        },
        {
            Name:        "digest",
            Description: "Generate a news digest",
            Options: []*discordgo.ApplicationCommandOption{
                {
                    Type:        discordgo.ApplicationCommandOptionString,
                    Name:        "timeframe",
                    Description: "Timeframe for the digest",
                    Required:    false,
                    Choices: []*discordgo.ApplicationCommandOptionChoice{
                        {Name: "Today", Value: "today"},
                        {Name: "Yesterday", Value: "yesterday"},
                        {Name: "Week", Value: "week"},
                    },
                },
            },
        },
        {
            Name:        "sources",
            Description: "Manage news sources",
            Options: []*discordgo.ApplicationCommandOption{
                {
                    Type:        discordgo.ApplicationCommandOptionString,
                    Name:        "action",
                    Description: "Action to perform",
                    Required:    true,
                    Choices: []*discordgo.ApplicationCommandOptionChoice{
                        {Name: "List", Value: "list"},
                        {Name: "Add", Value: "add"},
                        {Name: "Remove", Value: "remove"},
                        {Name: "Enable", Value: "enable"},
                        {Name: "Disable", Value: "disable"},
                    },
                },
            },
        },
        {
            Name:        "ping",
            Description: "Check bot latency",
        },
        {
            Name:        "status",
            Description: "Show bot status and statistics",
        },
    }
)

// registerCommands registers all slash commands
func (b *Bot) registerCommands() error {
    b.logger.Info("Registering commands...")
    
    for _, cmd := range commands {
        _, err := b.discord.ApplicationCommandCreate(b.discord.State.User.ID, "", cmd)
        if err != nil {
            return fmt.Errorf("failed to create command %s: %v", cmd.Name, err)
        }
    }
    
    return nil
}

// Command handlers

func (b *Bot) handleNewsCommand(s *discordgo.Session, i *discordgo.InteractionCreate) error {
    // Acknowledge the interaction immediately
    err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
        Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
    })
    if err != nil {
        return fmt.Errorf("failed to acknowledge interaction: %v", err)
    }

    // Get command options
    options := i.ApplicationCommandData().Options
    var category string
    if len(options) > 0 {
        category = options[0].StringValue()
    }

    // Fetch latest articles
    var articles []*NewsArticle
    if category != "" {
        // Fetch articles for specific category
        articles, err = b.database.GetArticlesByCategory(category, 10)
    } else {
        // Fetch latest articles across all categories
        articles, err = b.database.GetLatestArticles(10)
    }

    if err != nil {
        return fmt.Errorf("failed to fetch articles: %v", err)
    }

    // Format articles into embeds
    messages := b.formatter.FormatNewsDigest(articles)

    // Send response
    for _, msg := range messages {
        _, err = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
            Embeds: &msg.Embeds,
        })
        if err != nil {
            return fmt.Errorf("failed to send response: %v", err)
        }
    }

    return nil
}

func (b *Bot) handleDigestCommand(s *discordgo.Session, i *discordgo.InteractionCreate) error {
    // Acknowledge the interaction
    err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
        Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
    })
    if err != nil {
        return fmt.Errorf("failed to acknowledge interaction: %v", err)
    }

    // Get timeframe option
    options := i.ApplicationCommandData().Options
    timeframe := "today"
    if len(options) > 0 {
        timeframe = options[0].StringValue()
    }

    // Calculate time range
    var startTime time.Time
    switch timeframe {
    case "today":
        startTime = time.Now().UTC().Truncate(24 * time.Hour)
    case "yesterday":
        startTime = time.Now().UTC().Truncate(24 * time.Hour).Add(-24 * time.Hour)
    case "week":
        startTime = time.Now().UTC().Truncate(24 * time.Hour).Add(-7 * 24 * time.Hour)
    }

    // Fetch articles within timeframe
    articles, err := b.database.GetArticlesByTimeRange(startTime, time.Now().UTC())
    if err != nil {
        return fmt.Errorf("failed to fetch articles: %v", err)
    }

    // Generate digest
    messages := b.formatter.FormatNewsDigest(articles)

    // Send digest
    for _, msg := range messages {
        _, err = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
            Embeds: &msg.Embeds,
        })
        if err != nil {
            return fmt.Errorf("failed to send digest: %v", err)
        }
    }

    return nil
}

func (b *Bot) handleSourcesCommand(s *discordgo.Session, i *discordgo.InteractionCreate) error {
    options := i.ApplicationCommandData().Options
    if len(options) == 0 {
        return fmt.Errorf("no action specified")
    }

    action := options[0].StringValue()

    switch action {
    case "list":
        return b.handleListSources(s, i)
    case "add":
        return b.handleAddSource(s, i)
    case "remove":
        return b.handleRemoveSource(s, i)
    case "enable":
        return b.handleEnableSource(s, i)
    case "disable":
        return b.handleDisableSource(s, i)
    default:
        return fmt.Errorf("unknown action: %s", action)
    }
}

func (b *Bot) handleStatusCommand(s *discordgo.Session, i *discordgo.InteractionCreate) error {
    // Create status embed
    embed := &discordgo.MessageEmbed{
        Title: "📊 Bot Status",
        Color: 0x7289DA,
        Fields: []*discordgo.MessageEmbedField{
            {
                Name:   "Uptime",
                Value:  b.GetUptime().Round(time.Second).String(),
                Inline: true,
            },
            {
                Name:   "Version",
                Value:  VERSION,
                Inline: true,
            },
            {
                Name:   "Memory Usage",
                Value:  "TODO", // Implement memory usage tracking
                Inline: true,
            },
        },
        Timestamp: time.Now().Format(time.RFC3339),
    }

    // Add source statistics
    sources, err := b.database.GetSources()
    if err == nil {
        var active int
        for _, source := range sources {
            if !source.Paused {
                active++
            }
        }
        embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
            Name:   "News Sources",
            Value:  fmt.Sprintf("Active: %d\nTotal: %d", active, len(sources)),
            Inline: true,
        })
    }

    // Send response
    return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
        Type: discordgo.InteractionResponseChannelMessageWithSource,
        Data: &discordgo.InteractionResponseData{
            Embeds: []*discordgo.MessageEmbed{embed},
        },
    })
}

// Source management handlers

func (b *Bot) handleListSources(s *discordgo.Session, i *discordgo.InteractionCreate) error {
    sources, err := b.database.GetSources()
    if err != nil {
        return fmt.Errorf("failed to fetch sources: %v", err)
    }

    // Group sources by category
    categories := make(map[string][]*NewsSource)
    for _, source := range sources {
        categories[source.Category] = append(categories[source.Category], source)
    }

    // Create embed
    embed := &discordgo.MessageEmbed{
        Title: "📰 News Sources",
        Color: 0x7289DA,
    }

    // Add fields for each category
    for category, sources := range categories {
        var sourceList strings.Builder
        for _, source := range sources {
            status := "✅"
            if source.Paused {
                status = "❌"
            }
            sourceList.WriteString(fmt.Sprintf("%s %s\n", status, source.Name))
        }

        embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
            Name:   category,
            Value:  sourceList.String(),
            Inline: false,
        })
    }

    return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
        Type: discordgo.InteractionResponseChannelMessageWithSource,
        Data: &discordgo.InteractionResponseData{
            Embeds: []*discordgo.MessageEmbed{embed},
        },
    })
}

// TODO: Implement other source management handlers
func (b *Bot) handleAddSource(s *discordgo.Session, i *discordgo.InteractionCreate) error {
    return fmt.Errorf("not implemented")
}

func (b *Bot) handleRemoveSource(s *discordgo.Session, i *discordgo.InteractionCreate) error {
    return fmt.Errorf("not implemented")
}

func (b *Bot) handleEnableSource(s *discordgo.Session, i *discordgo.InteractionCreate) error {
    return fmt.Errorf("not implemented")
}

func (b *Bot) handleDisableSource(s *discordgo.Session, i *discordgo.InteractionCreate) error {
    return fmt.Errorf("not implemented")
}
// Adding to the existing commands.go file

// handleAddSource handles adding a new news source
func (b *Bot) handleAddSource(s *discordgo.Session, i *discordgo.InteractionCreate) error {
    // Check for required options
    options := i.ApplicationCommandData().Options
    if len(options) < 3 { // We need name, URL, and category
        return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
            Type: discordgo.InteractionResponseChannelMessageWithSource,
            Data: &discordgo.InteractionResponseData{
                Content: "❌ Required parameters: name, URL, and category",
                Flags:   discordgo.MessageFlagsEphemeral,
            },
        })
    }

    // Create new source
    source := &NewsSource{
        Name:      options[1].StringValue(),
        URL:       options[2].StringValue(),
        Category:  options[3].StringValue(),
        FactCheck: true, // Enable fact-checking by default
        Paused:    false,
    }

    // Validate URL format
    if !isValidURL(source.URL) {
        return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
            Type: discordgo.InteractionResponseChannelMessageWithSource,
            Data: &discordgo.InteractionResponseData{
                Content: "❌ Invalid URL format",
                Flags:   discordgo.MessageFlagsEphemeral,
            },
        })
    }

    // Validate category
    if !isValidCategory(source.Category) {
        return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
            Type: discordgo.InteractionResponseChannelMessageWithSource,
            Data: &discordgo.InteractionResponseData{
                Content: fmt.Sprintf("❌ Invalid category. Valid categories: %s", strings.Join(getValidCategories(), ", ")),
                Flags:   discordgo.MessageFlagsEphemeral,
            },
        })
    }

    // Save to database
    if err := b.database.SaveSource(source); err != nil {
        b.logger.Error("Failed to save source: %v", err)
        return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
            Type: discordgo.InteractionResponseChannelMessageWithSource,
            Data: &discordgo.InteractionResponseData{
                Content: "❌ Failed to save source",
                Flags:   discordgo.MessageFlagsEphemeral,
            },
        })
    }

    // Create success embed
    embed := &discordgo.MessageEmbed{
        Title:       "✅ News Source Added",
        Color:       0x00FF00,
        Description: fmt.Sprintf("Successfully added %s to the %s category", source.Name, source.Category),
        Fields: []*discordgo.MessageEmbedField{
            {
                Name:   "Name",
                Value:  source.Name,
                Inline: true,
            },
            {
                Name:   "Category",
                Value:  source.Category,
                Inline: true,
            },
            {
                Name:   "URL",
                Value:  source.URL,
                Inline: false,
            },
        },
        Timestamp: time.Now().Format(time.RFC3339),
    }

    return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
        Type: discordgo.InteractionResponseChannelMessageWithSource,
        Data: &discordgo.InteractionResponseData{
            Embeds: []*discordgo.MessageEmbed{embed},
        },
    })
}

// handleRemoveSource handles removing a news source
func (b *Bot) handleRemoveSource(s *discordgo.Session, i *discordgo.InteractionCreate) error {
    options := i.ApplicationCommandData().Options
    if len(options) < 1 {
        return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
            Type: discordgo.InteractionResponseChannelMessageWithSource,
            Data: &discordgo.InteractionResponseData{
                Content: "❌ Please specify the source name to remove",
                Flags:   discordgo.MessageFlagsEphemeral,
            },
        })
    }

    sourceName := options[1].StringValue()

    // Remove from database
    if err := b.database.DeleteSource(sourceName); err != nil {
        b.logger.Error("Failed to remove source: %v", err)
        return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
            Type: discordgo.InteractionResponseChannelMessageWithSource,
            Data: &discordgo.InteractionResponseData{
                Content: "❌ Failed to remove source",
                Flags:   discordgo.MessageFlagsEphemeral,
            },
        })
    }

    return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
        Type: discordgo.InteractionResponseChannelMessageWithSource,
        Data: &discordgo.InteractionResponseData{
            Content: fmt.Sprintf("✅ Successfully removed source: %s", sourceName),
        },
    })
}

// handleEnableSource handles enabling a paused news source
func (b *Bot) handleEnableSource(s *discordgo.Session, i *discordgo.InteractionCreate) error {
    options := i.ApplicationCommandData().Options
    if len(options) < 1 {
        return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
            Type: discordgo.InteractionResponseChannelMessageWithSource,
            Data: &discordgo.InteractionResponseData{
                Content: "❌ Please specify the source name to enable",
                Flags:   discordgo.MessageFlagsEphemeral,
            },
        })
    }

    sourceName := options[1].StringValue()

    // Get source from database
    source, err := b.database.GetSource(sourceName)
    if err != nil {
        return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
            Type: discordgo.InteractionResponseChannelMessageWithSource,
            Data: &discordgo.InteractionResponseData{
                Content: "❌ Source not found",
                Flags:   discordgo.MessageFlagsEphemeral,
            },
        })
    }

    // Update source status
    source.Paused = false
    if err := b.database.SaveSource(source); err != nil {
        b.logger.Error("Failed to enable source: %v", err)
        return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
            Type: discordgo.InteractionResponseChannelMessageWithSource,
            Data: &discordgo.InteractionResponseData{
                Content: "❌ Failed to enable source",
                Flags:   discordgo.MessageFlagsEphemeral,
            },
        })
    }

    return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
        Type: discordgo.InteractionResponseChannelMessageWithSource,
        Data: &discordgo.InteractionResponseData{
            Content: fmt.Sprintf("✅ Successfully enabled source: %s", sourceName),
        },
    })
}

// handleDisableSource handles pausing a news source
func (b *Bot) handleDisableSource(s *discordgo.Session, i *discordgo.InteractionCreate) error {
    options := i.ApplicationCommandData().Options
    if len(options) < 1 {
        return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
            Type: discordgo.InteractionResponseChannelMessageWithSource,
            Data: &discordgo.InteractionResponseData{
                Content: "❌ Please specify the source name to disable",
                Flags:   discordgo.MessageFlagsEphemeral,
            },
        })
    }

    sourceName := options[1].StringValue()

    // Get source from database
    source, err := b.database.GetSource(sourceName)
    if err != nil {
        return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
            Type: discordgo.InteractionResponseChannelMessageWithSource,
            Data: &discordgo.InteractionResponseData{
                Content: "❌ Source not found",
                Flags:   discordgo.MessageFlagsEphemeral,
            },
        })
    }

    // Update source status
    source.Paused = true
    if err := b.database.SaveSource(source); err != nil {
        b.logger.Error("Failed to disable source: %v", err)
        return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
            Type: discordgo.InteractionResponseChannelMessageWithSource,
            Data: &discordgo.InteractionResponseData{
                Content: "❌ Failed to disable source",
                Flags:   discordgo.MessageFlagsEphemeral,
            },
        })
    }

    return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
        Type: discordgo.InteractionResponseChannelMessageWithSource,
        Data: &discordgo.InteractionResponseData{
            Content: fmt.Sprintf("✅ Successfully disabled source: %s", sourceName),
        },
    })
}

// Helper functions

func isValidURL(url string) bool {
    // Basic URL validation
    return strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")
}

func isValidCategory(category string) bool {
    validCategories := getValidCategories()
    for _, valid := range validCategories {
        if category == valid {
            return true
        }
    }
    return false
}

func getValidCategories() []string {
    return []string{
        CategoryTechnology,
        CategoryBusiness,
        CategoryScience,
        CategoryHealth,
        CategoryPolitics,
        CategorySports,
        CategoryWorld,
    }
}
