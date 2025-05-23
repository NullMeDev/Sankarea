// cmd/sankarea/scheduler.go
package main

import (
    "context"
    "fmt"
    "sync"
    "time"

    "github.com/bwmarrin/discordgo"
)

// Stats represents bot statistics
type Stats struct {
    ArticleCount   int64
    ActiveSources  int
    LastUpdate     time.Time
    LastError      string
    ErrorCount     int64
}

// Source represents a news source configuration
type Source struct {
    Name     string `yaml:"name"`
    URL      string `yaml:"url"`
    Category string `yaml:"category"`
    Enabled  bool   `yaml:"enabled"`
}

// NewsArticle represents a processed news article
type NewsArticle struct {
    Title       string
    URL         string
    Description string
    ImageURL    string
    Category    string
    SourceName  string
    PublishedAt time.Time
}

// Scheduler handles periodic news feed checks
type Scheduler struct {
    bot        *Bot
    sources    []Source
    stats      Stats
    ticker     *time.Ticker
    done       chan bool
    mutex      sync.RWMutex
    interval   time.Duration
    processor  *NewsProcessor
    lastCheck  map[string]time.Time
}

// NewScheduler creates a new scheduler instance
func NewScheduler(bot *Bot, interval time.Duration) *Scheduler {
    return &Scheduler{
        bot:       bot,
        done:      make(chan bool),
        interval:  interval,
        processor: NewNewsProcessor(),
        lastCheck: make(map[string]time.Time),
    }
}

// Start begins the scheduling of feed checks
func (s *Scheduler) Start() error {
    // Load initial sources
    if err := s.LoadSources(); err != nil {
        return err
    }

    s.ticker = time.NewTicker(s.interval)
    
    go func() {
        // Initial check
        if err := s.checkFeeds(); err != nil {
            s.bot.logger.Error("Initial feed check failed: %v", err)
        }

        for {
            select {
            case <-s.ticker.C:
                if err := s.checkFeeds(); err != nil {
                    s.bot.logger.Error("Failed to check feeds: %v", err)
                }
            case <-s.done:
                return
            }
        }
    }()
    
    return nil
}

// Stop halts the scheduler
func (s *Scheduler) Stop() {
    if s.ticker != nil {
        s.ticker.Stop()
    }
    s.done <- true
}

// GetSources returns the list of configured sources
func (s *Scheduler) GetSources() []Source {
    s.mutex.RLock()
    defer s.mutex.RUnlock()
    return s.sources
}

// GetStats returns current statistics
func (s *Scheduler) GetStats() Stats {
    s.mutex.RLock()
    defer s.mutex.RUnlock()
    return s.stats
}

// RefreshNow triggers an immediate feed check
func (s *Scheduler) RefreshNow() error {
    return s.checkFeeds()
}

// checkFeeds performs the actual feed checking
func (s *Scheduler) checkFeeds() error {
    s.mutex.Lock()
    defer s.mutex.Unlock()

    // Create context with timeout
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
    defer cancel()

    // Process feeds
    articles, err := s.processor.ProcessFeeds(ctx, s.sources)
    if err != nil {
        s.stats.LastError = err.Error()
        s.stats.ErrorCount++
        return err
    }

    // Update stats
    s.stats.LastUpdate = time.Now()
    s.stats.ArticleCount += int64(len(articles))
    s.stats.ActiveSources = len(s.sources)

    // Update last check times
    for _, source := range s.sources {
        s.lastCheck[source.URL] = time.Now()
    }

    // Post articles to appropriate channels
    for _, article := range articles {
        if err := s.postArticle(article); err != nil {
            s.bot.logger.Error("Failed to post article: %v", err)
            continue
        }
        // Add small delay between posts to avoid rate limiting
        time.Sleep(time.Second)
    }

    return nil
}

// LoadSources loads sources from configuration
func (s *Scheduler) LoadSources() error {
    s.mutex.Lock()
    defer s.mutex.Unlock()

    // Load sources from config file
    sources, err := LoadSourcesConfig(s.bot.config.SourcesPath)
    if err != nil {
        return fmt.Errorf("failed to load sources: %v", err)
    }

    // Filter enabled sources and validate categories
    var enabledSources []Source
    for _, source := range sources {
        if !source.Enabled {
            continue
        }
        
        // Validate category
        validCategory := false
        for _, cat := range s.bot.config.Categories {
            if source.Category == cat {
                validCategory = true
                break
            }
        }
        
        if !validCategory {
            s.bot.logger.Warn("Invalid category for source %s: %s", source.Name, source.Category)
            continue
        }
        
        enabledSources = append(enabledSources, source)
    }

    s.sources = enabledSources
    s.stats.ActiveSources = len(enabledSources)
    return nil
}

// postArticle sends an article to the appropriate Discord channel
func (s *Scheduler) postArticle(article *NewsArticle) error {
    // Get channel ID for the article's category
    channelID := s.bot.config.CategoryChannels[article.Category]
    if channelID == "" {
        return fmt.Errorf("no channel configured for category: %s", article.Category)
    }

    // Create embed
    embed := &discordgo.MessageEmbed{
        Title:       article.Title,
        URL:         article.URL,
        Description: article.Description,
        Timestamp:   article.PublishedAt.Format(time.RFC3339),
        Color:       0x7289DA,
        Footer: &discordgo.MessageEmbedFooter{
            Text: fmt.Sprintf("Source: %s", article.SourceName),
        },
    }

    // Add image if available
    if article.ImageURL != "" {
        embed.Image = &discordgo.MessageEmbedImage{
            URL: article.ImageURL,
        }
    }

    // Send message
    _, err := s.bot.discord.ChannelMessageSendEmbed(channelID, embed)
    return err
}
