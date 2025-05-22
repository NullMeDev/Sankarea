package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
)

// FactCheckResult represents a fact checking result
type FactCheckResult struct {
	Claim        string  `json:"claim"`
	Rating       string  `json:"rating"`
	Source       string  `json:"source"`
	Explanation  string  `json:"explanation"`
	URL          string  `json:"url"`
	TrustScore   float64 `json:"trust_score"` // 0 to 1
	Method       string  `json:"method"`      // Which API was used
	CheckedAt    time.Time `json:"checked_at"`
}

// GoogleFactCheckResult represents a Google Fact Check Tools API result
type GoogleFactCheckResult struct {
	Claims []struct {
		Text      string `json:"text"`
		ClaimDate string `json:"claimDate"`
		ClaimReviewed string `json:"claimReviewed"`
		ClaimRating struct {
			TextualRating string  `json:"textualRating"`
			RatingValue   float64 `json:"ratingValue"`
		} `json:"claimRating"`
		ClaimReviewers []struct {
			Name string `json:"name"`
			URL  string `json:"url"`
		} `json:"claimReviewers"`
	} `json:"claims"`
}

// ClaimBustersResult represents a ClaimBusters API result
type ClaimBustersResult struct {
	ClaimScore  float64 `json:"score"`
	Explanation string  `json:"explanation"`
	IsFactual   bool    `json:"is_factual"`
	Sources     []struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"sources"`
	Rating      string  `json:"rating"`
}

// FactCheckArticle performs fact checking on an article
func FactCheckArticle(title, content, articleURL string) (*FactCheckResult, error) {
	// Make sure fact checking is enabled
	if cfg == nil || !cfg.EnableFactCheck {
		return nil, fmt.Errorf("Fact checking is disabled")
	}
	
	// Extract main claim from title and content
	claim := title
	if content != "" {
		// Add first sentence of content if available
		firstSentence := extractFirstSentence(content)
		if firstSentence != "" {
			claim = title + ". " + firstSentence
		}
	}
	
	// Try Google Fact Check API first
	if cfg.GoogleFactCheckAPIKey != "" {
		result, err := checkWithGoogleFactCheck(claim)
		if err == nil && result != nil {
			return result, nil
		}
		Logger().Printf("Google Fact Check failed: %v, falling back to other methods", err)
	}
	
	// Then try ClaimBusters
	if cfg.ClaimBustersAPIKey != "" {
		result, err := checkWithClaimBusters(claim)
		if err == nil && result != nil {
			return result, nil
		}
		Logger().Printf("ClaimBusters check failed: %v, falling back to other methods", err)
	}
	
	// Finally try OpenAI
	if cfg.OpenAIAPIKey != "" {
		result, err := checkWithOpenAI(claim, articleURL)
		if err == nil {
			return result, nil
		}
		Logger().Printf("OpenAI fact check failed: %v", err)
	}
	
	// If all methods failed
	return nil, fmt.Errorf("All fact checking methods failed")
}

// extractFirstSentence extracts the first sentence from a text
func extractFirstSentence(text string) string {
	// Simple sentence extraction - look for period followed by space or end
	parts := strings.SplitN(text, ". ", 2)
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

// checkWithGoogleFactCheck performs fact checking using Google Fact Check API
func checkWithGoogleFactCheck(claim string) (*FactCheckResult, error) {
	// Create a URL-safe version of the claim
	encodedClaim := url.QueryEscape(claim)
	
	// Build the API request URL
	apiURL := fmt.Sprintf(
		"https://factchecktools.googleapis.com/v1alpha1/claims:search?query=%s&key=%s",
		encodedClaim,
		cfg.GoogleFactCheckAPIKey,
	)
	
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	
	// Make the request
	resp, err := client.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	// Check response status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Google Fact Check API returned status: %s", resp.Status)
	}
	
	// Read and parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	
	var result GoogleFactCheckResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	
	// Check if we have any claims
	if len(result.Claims) == 0 {
		return nil, fmt.Errorf("No fact check results found")
	}
	
	// Get the first claim result
	firstClaim := result.Claims[0]
	
	// Create a fact check result
	factCheck := &FactCheckResult{
		Claim:       firstClaim.ClaimReviewed,
		Rating:      firstClaim.ClaimRating.TextualRating,
		TrustScore:  firstClaim.ClaimRating.RatingValue / 5.0, // Normalize to 0-1
		Method:      "Google Fact Check Tools API",
		CheckedAt:   time.Now(),
	}
	
	// Add source if available
	if len(firstClaim.ClaimReviewers) > 0 {
		factCheck.Source = firstClaim.ClaimReviewers[0].Name
		factCheck.URL = firstClaim.ClaimReviewers[0].URL
	}
	
	return factCheck, nil
}

// checkWithClaimBusters performs fact checking using ClaimBusters API
func checkWithClaimBusters(claim string) (*FactCheckResult, error) {
	// Build API request URL
	apiURL := "https://idir.uta.edu/claimbuster/api/v2/score/text/"
	
	// Create request body
	reqBody := map[string]string{
		"text": claim,
	}
	
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	
	// Create request
	req, err := http.NewRequest("POST", apiURL, strings.NewReader(string(jsonData)))
	if err != nil {
		return nil, err
	}
	
	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", cfg.ClaimBustersAPIKey)
	
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	
	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	// Check response status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ClaimBusters API returned status: %s", resp.Status)
	}
	
	// Read and parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	
	var result ClaimBustersResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	
	// Create fact check result
	factCheck := &FactCheckResult{
		Claim:       claim,
		Rating:      result.Rating,
		Explanation: result.Explanation,
		TrustScore:  result.ClaimScore,
		Method:      "ClaimBusters API",
		CheckedAt:   time.Now(),
	}
	
	// Add source if available
	if len(result.Sources) > 0 {
		factCheck.Source = result.Sources[0].Name
		factCheck.URL = result.Sources[0].URL
	}
	
	return factCheck, nil
}

// checkWithOpenAI performs fact checking using OpenAI API
func checkWithOpenAI(claim string, articleURL string) (*FactCheckResult, error) {
	if cfg.OpenAIAPIKey == "" {
		return nil, fmt.Errorf("OpenAI API key not configured")
	}
	
	client := openai.NewClient(cfg.OpenAIAPIKey)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	// Create prompt for fact checking
	systemPrompt := `You are a fact-checking AI assistant. Analyze the following claim and provide:
1. A factual rating (True, Mostly True, Mixed, Mostly False, False, Unverifiable)
2. A brief explanation of your rating
3. A trust score between 0.0 (completely false) and 1.0 (completely true)

Format your response as JSON with these fields:
{
  "rating": "rating here",
  "explanation": "explanation here",
  "trust_score": 0.5
}`

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
					Content: fmt.Sprintf("Fact check this claim: %s\nSource URL: %s", claim, articleURL),
				},
			},
			Temperature: 0.2,
			ResponseFormat: &openai.ChatCompletionResponseFormat{
				Type: openai.ChatCompletionResponseFormatTypeJSONObject,
			},
		},
	)
	if err != nil {
		return nil, err
	}

	// Parse response
	jsonResponse := resp.Choices[0].Message.Content
	jsonResponse = strings.TrimSpace(jsonResponse)

	// Parse the response JSON
	var result struct {
		Rating      string  `json:"rating"`
		Explanation string  `json:"explanation"`
		TrustScore  float64 `json:"trust_score"`
	}

	if err := json.Unmarshal([]byte(jsonResponse), &result); err != nil {
		return nil, fmt.Errorf("Failed to parse OpenAI response: %v", err)
	}

	// Create fact check result
	return &FactCheckResult{
		Claim:       claim,
		Rating:      result.Rating,
		Explanation: result.Explanation,
		TrustScore:  result.TrustScore,
		Method:      "OpenAI Analysis",
		CheckedAt:   time.Now(),
		URL:         articleURL,
	}, nil
}

// UpdateOpenAIUsageCost tracks OpenAI API usage (placeholder function)
func updateOpenAIUsageCost(tokens int) {
	// This function would be implemented to track API usage costs
	Logger().Printf("OpenAI API usage: %d tokens", tokens)
}
