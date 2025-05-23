// cmd/sankarea/factcheck.go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "strings"
    "sync"
    "time"
)

// FactChecker handles article fact-checking operations
type FactChecker struct {
    client          *http.Client
    mutex           sync.RWMutex
    cache           map[string]*FactCheckResult
    lastAPIRequests map[string]time.Time
}

// FactCheckResult represents the result of a fact check
type FactCheckResult struct {
    Score           float64   `json:"score"`           // 0-1 score where 1 is most reliable
    Claims          []Claim   `json:"claims"`          // Extracted claims
    Sources         []string  `json:"sources"`         // Fact-checking sources
    VerifiedAt      time.Time `json:"verified_at"`     // When the check was performed
    ExpiresAt       time.Time `json:"expires_at"`      // When this result should be re-checked
    FactCheckURL    string    `json:"fact_check_url"`  // URL to detailed fact check
    ReliabilityTier string    `json:"reliability_tier"`// High, Medium, Low reliability
}

// Claim represents a factual claim extracted from content
type Claim struct {
    Text       string    `json:"text"`
    Source     string    `json:"source"`
    CheckedAt  time.Time `json:"checked_at"`
    Rating     string    `json:"rating"`      // True, False, Partially True, etc.
    Evidence   string    `json:"evidence"`    // Supporting evidence
}

// NewFactChecker creates a new FactChecker instance
func NewFactChecker() *FactChecker {
    return &FactChecker{
        client: &http.Client{
            Timeout: DefaultTimeout,
        },
        cache:           make(map[string]*FactCheckResult),
        lastAPIRequests: make(map[string]time.Time),
    }
}

// CheckArticle performs fact-checking on a news article
func (fc *FactChecker) CheckArticle(ctx context.Context, article *NewsArticle) (*FactCheckResult, error) {
    // Check cache first
    if result := fc.getCachedResult(article.ID); result != nil {
        return result, nil
    }

    // Respect rate limits
    if !fc.canMakeRequest("factcheck") {
        return nil, NewFactCheckError(ErrRateLimit, "rate limit exceeded for fact-checking API", nil)
    }

    // Combine article content for analysis
    content := strings.Join([]string{
        article.Title,
        article.Summary,
    }, "\n")

    // Extract claims using ClaimBuster API
    claims, err := fc.extractClaims(ctx, content)
    if err != nil {
        return nil, NewFactCheckError(ErrClaimExtraction, "failed to extract claims", err)
    }

    // Check claims using Google Fact Check API
    verifiedClaims, err := fc.verifyClaims(ctx, claims)
    if err != nil {
        return nil, NewFactCheckError(ErrClaimVerification, "failed to verify claims", err)
    }

    // Calculate overall reliability score
    score := fc.calculateReliabilityScore(verifiedClaims)

    // Create result
    result := &FactCheckResult{
        Score:           score,
        Claims:          verifiedClaims,
        VerifiedAt:      time.Now(),
        ExpiresAt:       time.Now().Add(24 * time.Hour),
        ReliabilityTier: fc.getReliabilityTier(score),
    }

    // Cache result
    fc.cacheResult(article.ID, result)

    return result, nil
}

// extractClaims uses the ClaimBuster API to identify checkable claims
func (fc *FactChecker) extractClaims(ctx context.Context, content string) ([]Claim, error) {
    if cfg.ClaimBusterAPIKey == "" {
        return nil, NewFactCheckError(ErrConfiguration, "ClaimBuster API key not configured", nil)
    }

    url := fmt.Sprintf("https://idir.uta.edu/claimbuster/api/v2/score/text/%s", content)
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return nil, err
    }

    req.Header.Set("x-api-key", cfg.ClaimBusterAPIKey)
    
    resp, err := fc.client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var result struct {
        Results []struct {
            Text  string  `json:"text"`
            Score float64 `json:"score"`
        } `json:"results"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, err
    }

    // Convert to claims
    var claims []Claim
    for _, r := range result.Results {
        if r.Score >= 0.5 { // Only include likely claims
            claims = append(claims, Claim{
                Text:      r.Text,
                CheckedAt: time.Now(),
            })
        }
    }

    return claims, nil
}

// verifyClaims uses the Google Fact Check API to verify claims
func (fc *FactChecker) verifyClaims(ctx context.Context, claims []Claim) ([]Claim, error) {
    if cfg.GoogleFactCheckAPIKey == "" {
        return nil, NewFactCheckError(ErrConfiguration, "Google Fact Check API key not configured", nil)
    }

    for i := range claims {
        url := fmt.Sprintf("https://factchecktools.googleapis.com/v1alpha1/claims:search?key=%s&query=%s",
            cfg.GoogleFactCheckAPIKey,
            claims[i].Text,
        )

        req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
        if err != nil {
            return nil, err
        }

        resp, err := fc.client.Do(req)
        if err != nil {
            return nil, err
        }
        defer resp.Body.Close()

        var result struct {
            Claims []struct {
                Text         string `json:"text"`
                ClaimReview []struct {
                    Publisher struct {
                        Name string `json:"name"`
                    } `json:"publisher"`
                    TextualRating string `json:"textualRating"`
                    Url          string `json:"url"`
                } `json:"claimReview"`
            } `json:"claims"`
        }

        if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
            return nil, err
        }

        // Update claim with fact check results
        if len(result.Claims) > 0 && len(result.Claims[0].ClaimReview) > 0 {
            review := result.Claims[0].ClaimReview[0]
            claims[i].Rating = review.TextualRating
            claims[i].Source = review.Publisher.Name
            claims[i].Evidence = review.Url
        }
    }

    return claims, nil
}

// Helper functions

func (fc *FactChecker) calculateReliabilityScore(claims []Claim) float64 {
    if len(claims) == 0 {
        return 0.5 // Neutral score if no claims were checked
    }

    var totalScore float64
    for _, claim := range claims {
        score := fc.getRatingScore(claim.Rating)
        totalScore += score
    }

    return totalScore / float64(len(claims))
}

func (fc *FactChecker) getRatingScore(rating string) float64 {
    rating = strings.ToLower(rating)
    switch {
    case strings.Contains(rating, "true"):
        return 1.0
    case strings.Contains(rating, "mostly true"):
        return 0.75
    case strings.Contains(rating, "partially"):
        return 0.5
    case strings.Contains(rating, "mostly false"):
        return 0.25
    case strings.Contains(rating, "false"):
        return 0.0
    default:
        return 0.5
    }
}

func (fc *FactChecker) getReliabilityTier(score float64) string {
    switch {
    case score >= 0.8:
        return "High"
    case score >= 0.5:
        return "Medium"
    default:
        return "Low"
    }
}

func (fc *FactChecker) canMakeRequest(apiName string) bool {
    fc.mutex.Lock()
    defer fc.mutex.Unlock()

    lastRequest, exists := fc.lastAPIRequests[apiName]
    if !exists || time.Since(lastRequest) > time.Minute {
        fc.lastAPIRequests[apiName] = time.Now()
        return true
    }
    return false
}

func (fc *FactChecker) getCachedResult(articleID string) *FactCheckResult {
    fc.mutex.RLock()
    defer fc.mutex.RUnlock()

    if result, exists := fc.cache[articleID]; exists {
        if time.Now().Before(result.ExpiresAt) {
            return result
        }
        delete(fc.cache, articleID)
    }
    return nil
}

func (fc *FactChecker) cacheResult(articleID string, result *FactCheckResult) {
    fc.mutex.Lock()
    defer fc.mutex.Unlock()
    fc.cache[articleID] = result
}
