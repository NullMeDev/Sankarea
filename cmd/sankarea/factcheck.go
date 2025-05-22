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
	if !cfg.EnableFactCheck {
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

// extractFirstSentence extracts the first sentence from a string
func extractFirstSentence(text string) string {
	// Look for end of sentence markers
	endMarkers := []string{". ", "! ", "? "}
	
	for _, marker := range endMarkers {
		if idx := strings.Index(text, marker); idx > 0 {
			return text[:idx+1] // Include the period
		}
	}
	
	// If no markers found, limit to first 100 characters
	if len(text) > 100 {
		return text[:100] + "..."
	}
	
	return text
}

// checkWithGoogleFactCheck checks a claim with Google Fact Check Tools
func checkWithGoogleFactCheck(claim string) (*FactCheckResult, error) {
	// Prepare API call
	apiURL := "https://factchecktools.googleapis.com/v1alpha1/claims:search"
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	
	// Add query parameters
	q := req.URL.Query()
	q.Add("key", cfg.GoogleFactCheckAPIKey)
	q.Add("query", claim)
	req.URL.RawQuery = q.Encode()
	
	// Make the request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	// Check response status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Google API returned status code %d", resp.StatusCode)
	}
	
	// Parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	
	var result GoogleFactCheckResult
	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, err
	}
	
	// Process the result
	if len(result.Claims) == 0 {
		return nil, fmt.Errorf("No fact check results found")
	}
	
	// Get the first claim
	claim = result.Claims[0]
	
	// Map rating to trust score
	trustScore := 0.5 // Default neutral
	if claim.ClaimRating.RatingValue != 0 {
		// Normalize the rating to 0-1 scale
		trustScore = claim.ClaimRating.RatingValue / 5.0
	}
	
	// Create result
	factResult := &FactCheckResult{
		Claim:      claim.Text,
		Rating:     claim.ClaimRating.TextualRating,
		Source:     "Unknown",
		TrustScore: trustScore,
		Method:     "Google Fact Check",
		CheckedAt:  time.Now(),
	}
	
	// Add reviewer info if available
	if len(claim.ClaimReviewers) > 0 {
		factResult.Source = claim.ClaimReviewers[0].Name
		factResult.URL = claim.ClaimReviewers[0].URL
	}
	
	return factResult, nil
}

// checkWithClaimBusters checks a claim with ClaimBusters API
func checkWithClaimBusters(claim string) (*FactCheckResult, error) {
	apiURL := "https://idir.uta.edu/claimbuster/api/v2/score/text/"
	
	// Prepare API call
	encodedClaim := url.QueryEscape(claim)
	req, err := http.NewRequest("GET", apiURL+encodedClaim, nil)
	if err != nil {
		return nil, err
	}
	
	// Add API key header
	req.Header.Add("x-api-key", cfg.ClaimBustersAPIKey)
	
	// Make the request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	// Check response status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ClaimBusters API returned status code %d", resp.StatusCode)
	}
	
	// Parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	
	var result ClaimBustersResult
	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, err
	}
	
	// Map claim score to rating and trust score
	rating := "Unknown"
	trustScore := result.ClaimScore
	
	if trustScore > 0.8 {
		rating = "Highly Checkable Claim"
	} else if trustScore > 0.6 {
		rating = "Checkable Claim"
	} else if trustScore > 0.4 {
		rating = "Uncertain"
	} else if trustScore > 0.2 {
		rating = "Not Very Checkable"
	} else {
		rating = "Not a Claim"
	}
	
	// Create result
	factResult := &FactCheckResult{
		Claim:       claim,
		Rating:      rating,
		Source:      "ClaimBusters",
		Explanation: result.Explanation,
		TrustScore:  trustScore,
		Method:      "ClaimBusters",
		CheckedAt:   time.Now(),
	}
	
	// Add source info if available
	if len(result.Sources) > 0 {
		factResult.Source = result.Sources[0].Name
		factResult.URL = result.Sources[0].URL
	}
	
	return factResult, nil
}

// checkWithOpenAI uses OpenAI API to analyze claims in text
func checkWithOpenAI(claim, articleURL string) (*FactCheckResult, error) {
	if cfg.OpenAIAPIKey == "" {
		return nil, fmt.Errorf("OpenAI API key not configured")
	}
	
	client := openai.NewClient(cfg.OpenAIAPIKey)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	// Prepare system prompt for fact checking
	systemPrompt := `You are a fact checking assistant. Analyze the claim provided and determine:
1. Is it factually checkable (vs opinion)
2. Does it contain any obvious factual inaccuracies
3. Assign a rating from: "False", "Mostly False", "Mixed", "Mostly True", "True", or "Uncertain"
4. Provide a brief explanation of your rating

Format your response as JSON with these fields:
- rating: one of the ratings listed above
- explanation: brief explanation of why you gave this rating
- trust_score: a number between 0 (completely false) and 1 (completely true), or 0.5 if uncertain
- is_opinion: boolean indicating if this is primarily an opinion rather than a factual claim`

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
					Content: fmt.Sprintf("Analyze this claim: \"%s\"\nArticle URL: %s", claim, articleURL),
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
		Rating      string  `json:"rating"`
		Explanation string  `json:"explanation"`
		TrustScore  float64 `json:"trust_score"`
		IsOpinion   bool    `json:"is_opinion"`
	}

	err = json.Unmarshal([]byte(jsonResponse), &result)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse AI response: %v", err)
	}

	// Create fact check result
	factResult := &FactCheckResult{
		Claim:       claim,
		Rating:
