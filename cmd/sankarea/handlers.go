// cmd/sankarea/handlers.go
package main

import (
    "context"
    "fmt"
    "runtime"
    "strings"
    "time"

    "github.com/bwmarrin/discordgo"
)

var (
    buildDate = "2025-05-23"
    buildTime = "05:30:55"
)

// handlePingCommand handles the /ping command
func handlePingCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
    startTime := time.Now()
    
    // Send initial response
    err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
        Type: discordgo.InteractionResponseChannelMessageWithSource,
        Data: &discordgo.InteractionResponseData{
            Content: "🏓 Pinging...",
        },
    })
    if err != nil {
        Logger().Printf("Error responding to ping: %v", err)
        return
    }

    // Calculate latency
    latency := time.Since(startTime).Milliseconds()

    // Update message with latency
    _, err = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
        Content: fmt.Sprintf("🏓 Pong! Latency: %dms", latency),
    })
    if err != nil {
        Logger().Printf("Error updating ping response: %v", err)
    }
}

// handleStatusCommand handles the /status command
func handleStatusCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
    // Acknowledge interaction
    s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
        Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
    })

    // Load state and sources
    state, err := LoadState()
    if err != nil {
        followupWithError(s, i, "Failed to load state")
        return
    }

    sources, err := LoadSources()
    if err != nil {
        followupWithError(s, i, "Failed to load sources")
        return
    }

    // Build status message
    var sb strings.Builder
    sb.WriteString("**Sankarea Bot Status**\n\n")

    // General stats
    sb.WriteString("📊 **General**\n")
    sb.WriteString(fmt.Sprintf("• Uptime: %s\n", formatDuration(time.Since(state.StartupTime))))
    sb.WriteString(fmt.Sprintf("• Version: v%s\n", botVersion))
    sb.WriteString(fmt.Sprintf("• Health: %s\n", getHealthEmoji(state.HealthStatus)))
    sb.WriteString("\n")

    // News stats
    sb.WriteString("📰 **News**\n")
    sb.WriteString(fmt.Sprintf("• Active Sources: %d/%d\n", countActiveSources(sources), len(sources)))
    sb.WriteString(fmt.Sprintf("• Articles Fetched: %d\n", state.ArticleCount))
    sb.WriteString(fmt.Sprintf("• Last Fetch: %s\n", formatTimeAgo(state.LastFetchTime)))
    sb.WriteString(fmt.Sprintf("• Fetch Interval: %d minutes\n", cfg.NewsIntervalMinutes))
    sb.WriteString("\n")

    // Error stats
    sb.WriteString("⚠️ **Errors**\n")
    sb.WriteString(fmt.Sprintf("• Error Count: %d\n", state.ErrorCount))
    if state.LastError != "" {
        sb.WriteString(fmt.Sprintf("• Last Error: %s (%s ago)\n", 
            state.LastError, formatTimeAgo(state.LastErrorTime)))
    }

    // Send status message
    _, err = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
        Content: sb.String(),
    })
    if err != nil {
        Logger().Printf("Error sending status response: %v", err)
    }
}

// handleVersionCommand handles the /version command
func handleVersionCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
    buildDateTime := fmt.Sprintf("%s %s UTC", buildDate, buildTime)
    
    s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
        Type: discordgo.InteractionResponseChannelMessageWithSource,
        Data: &discordgo.InteractionResponseData{
            Embeds: []*discordgo.MessageEmbed{
                {
                    Title: "Bot Version Information",
                    Fields: []*discordgo.MessageEmbedField{
                        {
                            Name:   "Version",
                            Value:  fmt.Sprintf("v%s", botVersion),
                            Inline: true,
                        },
                        {
                            Name:   "Build Time",
                            Value:  buildDateTime,
                            Inline: true,
                        },
                        {
                            Name:   "Go Version",
                            Value:  runtime.Version(),
                            Inline: true,
                        },
                    },
                    Footer: &discordgo.MessageEmbedFooter{
                        Text: "Developed by NullMeDev",
                    },
                },
            },
        },
    })
}

// handleSourceCommand handles the /source command and its subcommands
func handleSourceCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
    options := i.ApplicationCommandData().Options
    if len(options) == 0 {
        respondWithError(s, i, "Invalid source command")
        return
    }

    switch options[0].Name {
    case "add":
        handleSourceAdd(s, i, options[0].Options)
    case "remove":
        handleSourceRemove(s, i, options[0].Options)
    case "list":
        handleSourceList(s, i)
    case "update":
        handleSourceUpdate(s, i, options[0].Options)
    default:
        respondWithError(s, i, "Unknown source subcommand")
    }
}

// handleSourceAdd handles the /source add subcommand
func handleSourceAdd(s *discordgo.Session, i *discordgo.InteractionCreate, options []*discordgo.ApplicationCommandInteractionDataOption) {
    // Acknowledge interaction
    s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
        Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
    })

    // Get parameters
    name := getOptionString(options, "name")
    url := getOptionString(options, "url")
    category := getOptionString(options, "category")
    factCheck := getOptionBool(options, "fact_check")

    // Validate source
    if err := validateSourceURL(url); err != nil {
        followupWithError(s, i, fmt.Sprintf("Invalid source URL: %v", err))
        return
    }

    // Create new source
    source := NewsSource{
        Name:      name,
        URL:       url,
        Category:  category,
        FactCheck: factCheck,
        Added:     time.Now(),
        AddedBy:   i.Member.User.ID,
    }

    // Add source
    sources, err := LoadSources()
    if err != nil {
        followupWithError(s, i, "Failed to load sources")
        return
    }

    // Check for duplicate
    for _, s := range sources {
        if strings.EqualFold(s.Name, name) {
            followupWithError(s, i, "A source with this name already exists")
            return
        }
    }

    // Add and save
    sources = append(sources, source)
    if err := SaveSources(sources); err != nil {
        followupWithError(s, i, "Failed to save sources")
        return
    }

    // Send success message
    s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
        Content: fmt.Sprintf("✅ Added source **%s** in category **%s**", name, category),
    })
}

// handleSourceList handles the /source list subcommand
func handleSourceList(s *discordgo.Session, i *discordgo.InteractionCreate) {
    s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
        Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
    })

    sources, err := LoadSources()
    if err != nil {
        followupWithError(s, i, "Failed to load sources")
        return
    }

    // Group sources by category
    categoryMap := make(map[string][]NewsSource)
    for _, source := range sources {
        categoryMap[source.Category] = append(categoryMap[source.Category], source)
    }

    // Build response
    var sb strings.Builder
    sb.WriteString("**📰 News Sources**\n\n")

    for category, sources := range categoryMap {
        sb.WriteString(fmt.Sprintf("**%s**\n", category))
        for _, source := range sources {
            status := "✅"
            if source.Paused {
                status = "⏸️"
            }
            sb.WriteString(fmt.Sprintf("%s %s - %s\n", status, source.Name, source.URL))
        }
        sb.WriteString("\n")
    }

    s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
        Content: sb.String(),
    })
}

// handleSourceRemove handles the /source remove subcommand
func handleSourceRemove(s *discordgo.Session, i *discordgo.InteractionCreate, options []*discordgo.ApplicationCommandInteractionDataOption) {
    s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
        Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
    })

    name := getOptionString(options, "name")
    if name == "" {
        followupWithError(s, i, "Please provide a source name")
        return
    }

    sources, err := LoadSources()
    if err != nil {
        followupWithError(s, i, "Failed to load sources")
        return
    }

    // Find and remove source
    found := false
    for idx, source := range sources {
        if strings.EqualFold(source.Name, name) {
            sources = append(sources[:idx], sources[idx+1:]...)
            found = true
            break
        }
    }

    if !found {
        followupWithError(s, i, "Source not found")
        return
    }

    if err := SaveSources(sources); err != nil {
        followupWithError(s, i, "Failed to save sources")
        return
    }

    s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
        Content: fmt.Sprintf("✅ Removed source **%s**", name),
    })
}

// handleSourceUpdate handles the /source update subcommand
func handleSourceUpdate(s *discordgo.Session, i *discordgo.InteractionCreate, options []*discordgo.ApplicationCommandInteractionDataOption) {
    s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
        Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
    })

    name := getOptionString(options, "name")
    if name == "" {
        followupWithError(s, i, "Please provide a source name")
        return
    }

    sources, err := LoadSources()
    if err != nil {
        followupWithError(s, i, "Failed to load sources")
        return
    }

    // Find and update source
    found := false
    for idx := range sources {
        if strings.EqualFold(sources[idx].Name, name) {
            // Update fields if provided
            if url := getOptionString(options, "url"); url != "" {
                if err := validateSourceURL(url); err != nil {
                    followupWithError(s, i, fmt.Sprintf("Invalid URL: %v", err))
                    return
                }
                sources[idx].URL = url
            }
            if category := getOptionString(options, "category"); category != "" {
                sources[idx].Category = category
            }
            if paused, ok := getOptionBoolValue(options, "paused"); ok {
                sources[idx].Paused = paused
            }
            found = true
            break
        }
    }

    if !found {
        followupWithError(s, i, "Source not found")
        return
    }

    if err := SaveSources(sources); err != nil {
        followupWithError(s, i, "Failed to save sources")
        return
    }

    s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
        Content: fmt.Sprintf("✅ Updated source **%s**", name),
    })
}

// Helper functions

func getOptionString(options []*discordgo.ApplicationCommandInteractionDataOption, name string) string {
    for _, opt := range options {
        if opt.Name == name {
            return opt.StringValue()
        }
    }
    return ""
}

func getOptionBool(options []*discordgo.ApplicationCommandInteractionDataOption, name string) bool {
    for _, opt := range options {
        if opt.Name == name {
            return opt.BoolValue()
        }
    }
    return false
}

func getOptionBoolValue(options []*discordgo.ApplicationCommandInteractionDataOption, name string) (bool, bool) {
    for _, opt := range options {
        if opt.Name == name {
            return opt.BoolValue(), true
        }
    }
    return false, false
}

func followupWithError(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
    s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
        Content: "❌ " + message,
    })
}

func formatDuration(d time.Duration) string {
    d = d.Round(time.Second)
    h := d / time.Hour
    d -= h * time.Hour
    m := d / time.Minute
    d -= m * time.Minute
    s := d / time.Second
    return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

func formatTimeAgo(t time.Time) string {
    if t.IsZero() {
        return "never"
    }
    
    d := time.Since(t)
    switch {
    case d < time.Minute:
        return "just now"
    case d < time.Hour:
        return fmt.Sprintf("%d minutes ago", int(d.Minutes()))
    case d < 24*time.Hour:
        return fmt.Sprintf("%d hours ago", int(d.Hours()))
    default:
        return fmt.Sprintf("%d days ago", int(d.Hours()/24))
    }
}

func getHealthEmoji(status string) string {
    switch status {
    case StatusOK:
        return "🟢 Healthy"
    case StatusDegraded:
        return "🟡 Degraded"
    default:
        return "🔴 Unhealthy"
    }
}

func countActiveSources(sources []NewsSource) int {
    count := 0
    for _, source := range sources {
        if !source.Paused {
            count++
        }
    }
    return count
}

func validateSourceURL(url string) error {
    if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
        return fmt.Errorf("URL must start with http:// or https://")
    }
    return nil
}
