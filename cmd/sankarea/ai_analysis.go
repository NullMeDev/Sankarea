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
	if cfg == nil || !cfg.EnableSummarization || cfg.OpenAIAPIKey == "" {
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
			ResponseFormat: &openai.ChatCompletionResponseFormat{
				Type: openai.ChatCompletionResponseFormatTypeJSONObject,
			},
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

	// Parse the response JSON
	var result struct {
		Sentiment     string             `json:"sentiment"`
		Score         float64            `json:"score"`
		Topics        []string           `json:"topics"`
		Keywords      []string           `json:"keywords"`
		EntityCount   map[string]int     `json:"entity_count"`
		IsOpinionated bool               `json:"is_opinionated"`
	}

	if err := json.Unmarshal([]byte(jsonResponse), &result); err != nil {
		return nil, fmt.Errorf("Failed to parse AI analysis: %v", err)
	}

	return &SentimentAnalysis{
		Sentiment:    result.Sentiment,
		Score:        result.Score,
		Topics:       result.Topics,
		Keywords:     result.Keywords,
		EntityCount:  result.EntityCount,
		IsOpinionated: result.IsOpinionated,
	}, nil
}

// Helper function min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// SummarizeArticle generates a concise summary of an article
func SummarizeArticle(article *Article, maxLength int) (string, error) {
	if cfg == nil || !cfg.EnableSummarization || cfg.OpenAIAPIKey == "" {
		return "", fmt.Errorf("OpenAI integration not configured")
	}

	client := openai.NewClient(cfg.OpenAIAPIKey)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	// Prepare system prompt for article summarization
	systemPrompt := fmt.Sprintf(`You are an AI that summarizes news articles in a neutral, factual manner.
Create a concise summary that captures the key points of the article.
Keep the summary under %d characters.
Do not include your own opinions or analysis.`, maxLength)

	// Extract a shorter version of the content for analysis
	contentToAnalyze := article.Title
	if len(article.Content) > 0 {
		// Take the first 2500 characters or so for summarization
		contentLen := min(len(article.Content), 2500)
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
					Content: fmt.Sprintf("Summarize this article from %s:\n\n%s", article.Source, contentToAnalyze),
				},
			},
			Temperature: 0.3, // Low temperature for more consistent results
		},
	)
	if err != nil {
		return "", fmt.Errorf("OpenAI API error: %v", err)
	}

	// Update API usage cost
	updateOpenAIUsageCost(resp.Usage.TotalTokens)

	// Extract summary
	summary := resp.Choices[0].Message.Content
	summary = strings.TrimSpace(summary)

	// Ensure summary doesn't exceed max length
	if len(summary) > maxLength {
		summary = summary[:maxLength] + "..."
	}

	return summary, nil
}
