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

// Continue adding after the previous command handlers

func handleFactCheckCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
    options := i.ApplicationCommandData().Options
    if len(options) < 1 {
        respondWithError(s, i, "Please provide a URL to fact-check")
        return
    }

    // Acknowledge the interaction immediately as fact-checking might take time
    s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
        Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
    })

    url := options[0].StringValue()
    factChecker := NewFactChecker()
    
    // Create a mock article for fact checking
    article := &NewsArticle{
        URL:       url,
        FetchedAt: time.Now(),
    }

    result, err := factChecker.CheckArticle(context.Background(), article)
    if err != nil {
        Logger().Printf("Fact-check error: %v", err)
        editResponse(s, i, "❌ Failed to perform fact-check")
        return
    }

    // Create fact-check embed
    embed := &discordgo.MessageEmbed{
        Title: "📊 Fact Check Results",
        Color: getReliabilityColor(result.ReliabilityTier),
        Fields: []*discordgo.MessageEmbedField{
            {
                Name:   "Reliability Score",
                Value:  fmt.Sprintf("%.2f/1.00", result.Score),
                Inline: true,
            },
            {
                Name:   "Reliability Tier",
                Value:  result.ReliabilityTier,
                Inline: true,
            },
        },
        URL:       url,
        Timestamp: time.Now().Format(time.RFC3339),
        Footer: &discordgo.MessageEmbedFooter{
            Text: fmt.Sprintf("Fact-checked by Sankarea v%s", VERSION),
        },
    }

    // Add claims if any
    if len(result.Claims) > 0 {
        claimsField := &discordgo.MessageEmbedField{
            Name:   "Verified Claims",
            Value:  "",
            Inline: false,
        }

        for _, claim := range result.Claims {
            claimsField.Value += fmt.Sprintf("• %s\n  Rating: %s\n", claim.Text, claim.Rating)
            if claim.Evidence != "" {
                claimsField.Value += fmt.Sprintf("  Evidence: %s\n", claim.Evidence)
            }
        }

        embed.Fields = append(embed.Fields, claimsField)
    }

    // Add reasoning if available
    if len(result.Reasons) > 0 {
        reasonsField := &discordgo.MessageEmbedField{
            Name:   "Analysis",
            Value:  strings.Join(result.Reasons, "\n"),
            Inline: false,
        }
        embed.Fields = append(embed.Fields, reasonsField)
    }

    editResponseWithEmbed(s, i, embed)
}

func handleDigestCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
    // Acknowledge the interaction
    s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
        Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
    })

    period := "daily" // default
    if len(i.ApplicationCommandData().Options) > 0 {
        period = i.ApplicationCommandData().Options[0].StringValue()
    }

    var startTime time.Time
    switch period {
    case "weekly":
        startTime = time.Now().UTC().AddDate(0, 0, -7)
    default: // daily
        startTime = time.Now().UTC().AddDate(0, 0, -1)
    }

    articles, err := state.DB.GetArticlesByTimeRange(startTime, time.Now().UTC())
    if err != nil {
        Logger().Printf("Failed to fetch articles for digest: %v", err)
        editResponse(s, i, "❌ Failed to generate digest")
        return
    }

    if len(articles) == 0 {
        editResponse(s, i, "ℹ️ No articles found for the selected period")
        return
    }

    // Group articles by category
    categories := make(map[string][]*NewsArticle)
    for _, article := range articles {
        categories[article.Category] = append(categories[article.Category], article)
    }

    // Create summary embed
    summaryEmbed := &discordgo.MessageEmbed{
        Title:       fmt.Sprintf("📰 %s News Digest", strings.Title(period)),
        Description: fmt.Sprintf("News summary for %s", time.Now().Format("2006-01-02 15:04 MST")),
        Color:       0x7289DA,
        Fields:      make([]*discordgo.MessageEmbedField, 0),
        Footer: &discordgo.MessageEmbedFooter{
            Text: fmt.Sprintf("Generated by Sankarea v%s | %s", VERSION, time.Now().Format(time.RFC3339)),
        },
    }

    // Send summary first
    for category, articles := range categories {
        summaryEmbed.Fields = append(summaryEmbed.Fields, &discordgo.MessageEmbedField{
            Name:   fmt.Sprintf("%s %s", getCategoryEmoji(category), category),
            Value:  fmt.Sprintf("%d articles", len(articles)),
            Inline: true,
        })
    }

    editResponseWithEmbed(s, i, summaryEmbed)

    // Send category details as follow-up messages
    for category, articles := range categories {
        embed := &discordgo.MessageEmbed{
            Title: fmt.Sprintf("%s %s News", getCategoryEmoji(category), category),
            Color: getCategoryColor(category),
            Fields: make([]*discordgo.MessageEmbedField, 0),
        }

        for _, article := range articles {
            reliability := "N/A"
            if article.FactCheckResult != nil {
                reliability = fmt.Sprintf("%s (%.2f)", 
                    article.FactCheckResult.ReliabilityTier, 
                    article.FactCheckResult.Score)
            }

            embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
                Name: article.Title,
                Value: fmt.Sprintf("Source: %s\nReliability: %s\n[Read More](%s)",
                    article.Source,
                    reliability,
                    article.URL),
                Inline: false,
            })
        }

        followUpMessage(s, i, embed)
    }
}

func handleHelpCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
    var embed *discordgo.MessageEmbed

    // Check if specific command help was requested
    if len(i.ApplicationCommandData().Options) > 0 {
        cmdName := i.ApplicationCommandData().Options[0].StringValue()
        embed = getCommandHelp(cmdName)
    } else {
        embed = getGeneralHelp()
    }

    s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
        Type: discordgo.InteractionResponseChannelMessageWithSource,
        Data: &discordgo.InteractionResponseData{
            Embeds: []*discordgo.MessageEmbed{embed},
        },
    })
}

// Helper functions

func getReliabilityColor(tier string) int {
    switch tier {
    case "High":
        return 0x00FF00 // Green
    case "Medium":
        return 0xFFFF00 // Yellow
    case "Low":
        return 0xFF0000 // Red
    default:
        return 0x808080 // Gray
    }
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
        return 0x00FF00 // Green
    case CategoryBusiness:
        return 0x0000FF // Blue
    case CategoryScience:
        return 0x9400D3 // Purple
    case CategoryHealth:
        return 0xFF0000 // Red
    case CategoryPolitics:
        return 0xFFA500 // Orange
    case CategorySports:
        return 0xFFFF00 // Yellow
    case CategoryWorld:
        return 0x4169E1 // Royal Blue
    default:
        return 0x808080 // Gray
    }
}

func editResponse(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
    s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
        Content: &content,
    })
}

func editResponseWithEmbed(s *discordgo.Session, i *discordgo.InteractionCreate, embed *discordgo.MessageEmbed) {
    s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
        Embeds: &[]*discordgo.MessageEmbed{embed},
    })
}

func followUpMessage(s *discordgo.Session, i *discordgo.InteractionCreate, embed *discordgo.MessageEmbed) {
    s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
        Embeds: []*discordgo.MessageEmbed{embed},
    })
}
