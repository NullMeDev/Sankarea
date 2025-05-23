// cmd/sankarea/models.go
package main

import (
    "time"
)

// Article represents parsed article content
type Article struct {
    Title     string
    Content   string
    URL       string
    Source    string
    Timestamp time.Time
    Category  string
    Sentiment float64
    FactScore float64
    Summary   string
}

// ArticleDigest represents a summarized article for digests
type ArticleDigest struct {
    Title     string    `json:"title"`
    URL       string    `json:"url"`
    Source    string    `json:"source"`
    Published time.Time `json:"published"`
    Category  string    `json:"category"`
    Bias      string    `json:"bias"`
}

// SentimentAnalysis contains sentiment analysis results
type SentimentAnalysis struct {
    Sentiment     string         `json:"sentiment"`
    Score         float64        `json:"score"`
    Topics        []string       `json:"topics"`
    Keywords      []string       `json:"keywords"`
    EntityCount   map[string]int `json:"entity_count"`
    IsOpinionated bool          `json:"is_opinionated"`
}

// Permission levels for command access
const (
    PermLevelEveryone = iota
    PermLevelAdmin
    PermLevelOwner
)
