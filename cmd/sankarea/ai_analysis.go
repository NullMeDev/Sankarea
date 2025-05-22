package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
)

// Article represents parsed article content
type Article struct {
	Title     string
	Content   string
	URL       string
	Source    string
	Timestamp time.Time
}

// SentimentAnalysis contains sentiment analysis results
type SentimentAnalysis struct {
	Sentiment    string  // positive, negative, neutral
	Score        float64 // -1.0 to 1.0
	Topics       []string
	Keywords     []string
	EntityCount  map[string]int // Named entities and their counts
	IsOpinionated bool
}

// AnalyzeArticleSentiment performs sentiment analysis on an article
func AnalyzeArticleSentiment(article *Article) (*SentimentAnalysis, error) {
	if !cfg.EnableSummarization || cfg.OpenAIAPIKey == "" {
		return nil, fmt.Errorf("OpenAI integration not configured")
	}

	client := openai.NewClient(cfg.OpenAIAPIKey)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	// Prepare system prompt for article analysis
	systemPrompt := `You are an AI that analyzes news articles for sentiment, topics, and key entities. 
Provide output in JSON format with these fields:
- sentiment: "positive", "negative", or "neutral"
- score: a number between -1.0 (very negative) and 1.0 (very positive)
- topics: an array of up to 5 main topics in the article
- keywords: an array of up to 10 important keywords
- entity_count: an object mapping named entities (people, organizations, places) to their counts in the article
- is_opinionated: boolean indicating if the article contains strong opinions rather than purely factual information`

	// Extract a shorter version of the content for analysis
	contentToAnalyze := article.Title
	if len(article.Content) > 0 {
		// Take the first 1500 characters or so for analysis
		contentLen := min(len(article.Content), 1500)
		contentToAnalyze += "\n\n" + article.Content[:contentLen]
	}

	// Create completion request
	resp, err := client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: "gpt-3.5-turbo",
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    "system",
					Content: systemPrompt,
				},
				{
					Role:    "user",
					Content: fmt.Sprintf("Analyze this article from %s:\n\n%s", article.Source, contentToAnalyze),
				},
			},
			Temperature: 0.2, // Low temperature for more consistent results
		},
	)
	if err != nil {
		return nil, fmt.Errorf("OpenAI API error: %v", err)
	}

	// Update API usage cost
	updateOpenAIUsageCost(resp.Usage.TotalTokens)

	// Parse the JSON response
	jsonResponse := resp.Choices[0].Message.Content
	jsonResponse = strings.TrimSpace(jsonResponse)

	// Sometimes GPT wraps results in code blocks, remove those
	jsonResponse = strings.TrimPrefix(jsonResponse, "```json")
	jsonResponse = strings.TrimPrefix(jsonResponse, "```")
	jsonResponse = strings.TrimSuffix(jsonResponse, "```")
	jsonResponse = strings.TrimSpace(jsonResponse)

	var result struct {
		Sentiment     string             `json:"sentiment"`
		Score         float64            `json:"score"`
		Topics        []string           `json:"topics"`
		Keywords      []string           `json:"keywords"`
		EntityCount   map[string]int     `json:"entity_count"`
		IsOpinionated bool               `json:"is_opinionated"`
	}

	err = json.Unmarshal([]byte(jsonResponse), &result)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse AI response: %v", err)
	}

	analysis := &SentimentAnalysis{
		Sentiment:    result.Sentiment,
		Score:        result.Score,
		Topics:       result.Topics,
		Keywords:     result.Keywords,
		EntityCount:  result.EntityCount,
		IsOpinionated: result.IsOpinionated,
	}

	return analysis, nil
}

// GetArticleCategoryRecommendation determines the best category for an article
func GetArticleCategoryRecommendation(sentiment *SentimentAnalysis) string {
	if len(sentiment.Topics) == 0 {
		return "General"
	}

	// Map common topics to categories
	topicCategories := map[string]string{
		"politics":    "Politics",
		"government":  "Politics",
		"election":    "Politics",
		"technology":  "Technology",
		"tech":        "Technology",
		"science":     "Science",
		"research":    "Science",
		"business":    "Business",
		"economy":     "Business",
		"finance":     "Business",
		"sports":      "Sports",
		"gaming":      "Entertainment",
		"movie":       "Entertainment",
		"film":        "Entertainment",
		"entertain":   "Entertainment",
		"health":      "Health",
		"medical":     "Health",
		"environment": "Environment",
		"climate":     "Environment",
		"culture":     "Culture",
		"arts":        "Culture",
		"education":   "Education",
		"world":       "World News",
		"war":         "World News",
		"conflict":    "World News",
	}

	// Check topics against our mapping
	for _, topic := range sentiment.Topics {
		topicLower := strings.ToLower(topic)
		for key, category := range topicCategories {
			if strings.Contains(topicLower, key) {
				return category
			}
		}
	}

	// Default category
	return "General"
}

// updateOpenAIUsageCost updates the usage cost in the state
func updateOpenAIUsageCost(tokens int) {
	// Approximate cost calculation: $0.002 per 1K tokens for gpt-3.5-turbo
	cost := float64(tokens) * 0.002 / 1000.0

	state, err := LoadState()
	if err != nil {
		Logger().Printf("Failed to load state for API cost tracking: %v", err)
		return
	}

	state.LastSummaryCost = cost
	state.DailyAPIUsage += cost
	
	err = SaveState(state)
	if err != nil {
		Logger().Printf("Failed to save state for API cost tracking: %v", err)
	}
}
