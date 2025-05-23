// cmd/sankarea/news.go
package main

import (
    "context"
    "fmt"
    "net/http"
    "sort"
    "strings"
    "sync"
    "time"

    "github.com/bwmarrin/discordgo"
    "github.com/mmcdole/gofeed"
)

// NewsProcessor handles news feed processing and Discord integration
type NewsProcessor struct {
    client    *http.Client
    parser    *gofeed.Parser
    mutex     sync.RWMutex
    cache     map[string]time.Time
    semaphore chan struct{}
}

// NewNewsProcessor creates a new NewsProcessor instance
func NewNewsProcessor() *NewsProcessor {
    return &NewsProcessor{
        client: &http.Client{
            Timeout: DefaultTimeout,
        },
        parser:    gofeed.NewParser(),
        cache:     make(map[string]time.Time),
        semaphore: make(chan struct{}, MaxConcurrentFeeds),
    }
}

// ProcessNews fetches and processes news from all sources
func (np *NewsProcessor) ProcessNews(ctx context.Context, s *discordgo.Session) error {
    startTime := time.Now()
    
    // Load active sources
    sources, err := LoadSources()
    if err != nil {
        return NewNewsError(ErrNewsFetch, "failed to load sources", err)
    }

    // Filter active sources
    activeSources := filterActiveSources(sources)
    if len(activeSources) == 0 {
        return NewNewsError(ErrNewsFetch, "no active sources configured", nil)
    }

    // Create error channel for collecting errors
    errCh := make(chan error, len(activeSources))
    articleCh := make(chan *NewsArticle, len(activeSources)*10)
    var wg sync.WaitGroup

    // Process each source
    for _, source := range activeSources {
        wg.Add(1)
        go func(src NewsSource) {
            defer wg.Done()
            defer RecoverFromPanic(fmt.Sprintf("news-fetch-%s", src.Name))

            if err := np.processFeed(ctx, src, articleCh); err != nil {
                errCh <- err
                updateSourceError(src.Name, err)
            }
        }(source)
    }

    // Wait for all fetches to complete
    go func() {
        wg.Wait()
        close(articleCh)
        close(errCh)
    }()

    // Collect and process articles
    var articles []*NewsArticle
    for article := range articleCh {
        articles = append(articles, article)
    }

    // Sort and filter articles
    articles = np.processArticles(articles)

    // Post articles to Discord
    if err := np.postArticles(s, articles); err != nil {
        return NewNewsError(ErrNewsParser, "failed to post articles", err)
    }

    // Collect errors
    var errors []error
    for err := range errCh {
        errors = append(errors, err)
    }

    // Update state after processing
    if err := UpdateState(func(s *State) {
        s.LastFetchTime = time.Now()
        s.ArticleCount += len(articles)
        s.LastInterval = int(time.Since(startTime).Minutes())
        if len(errors) > 0 {
            s.ErrorCount += len(errors)
            s.LastError = errors[0].Error()
            s.LastErrorTime = time.Now()
        }
    }); err != nil {
        Logger().Printf("Failed to update state after news fetch: %v", err)
    }

    if len(errors) > 0 {
        return fmt.Errorf("encountered %d errors while fetching news", len(errors))
    }

    return nil
}

// processFeed handles fetching and processing a single feed
func (np *NewsProcessor) processFeed(ctx context.Context, source NewsSource, articleCh chan<- *NewsArticle) error {
    // Acquire semaphore
    select {
    case np.semaphore <- struct{}{}:
        defer func() { <-np.semaphore }()
    case <-ctx.Done():
        return ctx.Err()
    }

    // Check cache to avoid duplicate processing
    np.mutex.RLock()
    lastFetch, exists := np.cache[source.URL]
    np.mutex.RUnlock()

    if exists && time.Since(lastFetch) < time.Duration(cfg.MinFetchInterval)*time.Minute {
        return nil
    }

    // Set user agent
    np.parser.UserAgent = cfg.UserAgentString

    // Fetch and parse feed
    feed, err := np.parser.ParseURL(source.URL)
    if err != nil {
        return NewNewsError(ErrNewsFetch, fmt.Sprintf("failed to parse feed for %s", source.Name), err)
    }

    // Update cache
    np.mutex.Lock()
    np.cache[source.URL] = time.Now()
    np.mutex.Unlock()

    // Process feed items
    for _, item := range feed.Items {
        article := convertFeedItemToArticle(item, source)
        articleCh <- article
    }

    return nil
}

// processArticles sorts and filters articles
func (np *NewsProcessor) processArticles(articles []*NewsArticle) []*NewsArticle {
    // Sort by publish date
    sort.Slice(articles, func(i, j int) bool {
        return articles[i].PublishedAt.After(articles[j].PublishedAt)
    })

    // Filter duplicates and old articles
    seen := make(map[string]bool)
    cutoff := time.Now().Add(-24 * time.Hour)
    filtered := make([]*NewsArticle, 0)

    for _, article := range articles {
        // Skip old articles
        if article.PublishedAt.Before(cutoff) {
            continue
        }

        // Skip duplicates (based on title similarity)
        if seen[normalizeTitle(article.Title)] {
            continue
        }
        seen[normalizeTitle(article.Title)] = true

        filtered = append(filtered, article)
    }

    return filtered
}

// postArticles posts articles to Discord channels
func (np *NewsProcessor) postArticles(s *discordgo.Session, articles []*NewsArticle) error {
    for _, guild := range cfg.Guilds {
        guildConfig, err := LoadGuildConfig(guild.ID)
        if err != nil {
            Logger().Printf("Error loading config for guild %s: %v", guild.ID, err)
            continue
        }

        // Group articles by category
        categoryArticles := groupArticlesByCategory(articles)

        // Post to appropriate channels
        for category, catArticles := range categoryArticles {
            channelID := getChannelForCategory(guildConfig, category)
            if channelID == "" {
                continue
            }

            for _, article := range catArticles {
                embed := createNewsEmbed(article)
                if _, err := s.ChannelMessageSendEmbed(channelID, embed); err != nil {
                    Logger().Printf("Error posting article to channel %s: %v", channelID, err)
                }
            }
        }
    }

    return nil
}

// Helper functions

func filterActiveSources(sources []NewsSource) []NewsSource {
    active := make([]NewsSource, 0)
    for _, source := range sources {
        if !source.Paused {
            active = append(active, source)
        }
    }
    return active
}

func convertFeedItemToArticle(item *gofeed.Item, source NewsSource) *NewsArticle {
    publishedAt := time.Now()
    if item.PublishedParsed != nil {
        publishedAt = *item.PublishedParsed
    }

    article := &NewsArticle{
        ID:          generateArticleID(item),
        Title:       item.Title,
        URL:         item.Link,
        Source:      source.Name,
        PublishedAt: publishedAt,
        FetchedAt:   time.Now(),
        Summary:     item.Description,
        Category:    source.Category,
        Tags:        item.Categories,
    }

    if item.Image != nil {
        article.ImageURL = item.Image.URL
    }

    return article
}

func createNewsEmbed(article *NewsArticle) *discordgo.MessageEmbed {
    embed := &discordgo.MessageEmbed{
        Title:       article.Title,
        URL:         article.URL,
        Description: truncateString(article.Summary, MaxEmbedLength),
        Timestamp:   article.PublishedAt.Format(time.RFC3339),
        Color:       getCategoryColor(article.Category),
        Footer: &discordgo.MessageEmbedFooter{
            Text: fmt.Sprintf("Source: %s", article.Source),
        },
    }

    if article.ImageURL != "" {
        embed.Image = &discordgo.MessageEmbedImage{
            URL: article.ImageURL,
        }
    }

    if len(article.Tags) > 0 {
        embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
            Name:   "Tags",
            Value:  strings.Join(article.Tags, ", "),
            Inline: true,
        })
    }

    return embed
}

func generateArticleID(item *gofeed.Item) string {
    if item.GUID != "" {
        return item.GUID
    }
    return fmt.Sprintf("%s-%s", normalizeTitle(item.Title), time.Now().Format("20060102"))
}

func normalizeTitle(title string) string {
    return strings.ToLower(strings.Join(strings.Fields(title), " "))
}

func groupArticlesByCategory(articles []*NewsArticle) map[string][]*NewsArticle {
    grouped := make(map[string][]*NewsArticle)
    for _, article := range articles {
        grouped[article.Category] = append(grouped[article.Category], article)
    }
    return grouped
}

func getChannelForCategory(config *GuildConfig, category string) string {
    if channelID := config.CategoryChannels[category]; channelID != "" {
        return channelID
    }
    return config.NewsChannel // fallback to default channel
}

func truncateString(s string, maxLen int) string {
    if len(s) <= maxLen {
        return s
    }
    return s[:maxLen-3] + "..."
}

func getCategoryColor(category string) int {
    colors := map[string]int{
        CategoryTechnology: 0x7289DA,
        CategoryBusiness:  0x43B581,
        CategoryScience:   0xFAA61A,
        CategoryHealth:    0xF04747,
        CategoryPolitics:  0x747F8D,
        CategorySports:    0x2ECC71,
        CategoryWorld:     0x99AAB5,
    }

    if color, ok := colors[category]; ok {
        return color
    }
    return 0x99AAB5 // default color
}
