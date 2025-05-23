// cmd/sankarea/factchecker.go
package main

import (
    "context"
    "fmt"
    "net/http"
    "strings"
    "sync"
    "time"
)

// FactChecker handles article reliability checking
type FactChecker struct {
    client    *http.Client
    cache     map[string]*FactCheckResult
    cacheMu   sync.RWMutex
    cacheTime time.Duration
}

// FactCheckResult represents the result of a fact check
type FactCheckResult struct {
    Score          float64  `json:"score"`
    ReliabilityTier string   `json:"reliability_tier"`
    Claims         []Claim  `json:"claims,omitempty"`
    Timestamp      time.Time `json:"timestamp"`
}

// Claim represents a verified claim in an article
type Claim struct {
    Text     string `json:"text"`
    Rating   string `json:"rating"`
    Evidence string `json:"evidence,omitempty"`
}

// NewFactChecker creates a new fact checker instance
func NewFactChecker() *FactChecker {
    return &FactChecker{
        client: &http.Client{
            Timeout: 30 * time.Second,
        },
        cache:     make(map[string]*FactCheckResult),
        cacheTime: 24 * time.Hour,
    }
}

// CheckArticle performs fact checking on an article
func (fc *FactChecker) CheckArticle(ctx context.Context, article *NewsArticle) (*FactCheckResult, error) {
    // Check cache first
    if result := fc.getCachedResult(article.URL); result != nil {
        return result, nil
    }

    // Perform reliability analysis
    score, err := fc.analyzeReliability(ctx, article)
    if err != nil {
        return nil, fmt.Errorf("reliability analysis failed: %v", err)
    }

    // Extract and verify claims
    claims, err := fc.extractClaims(ctx, article)
    if err != nil {
        Logger().Printf("Warning: claim extraction failed for %s: %v", article.URL, err)
        // Continue with empty claims rather than failing completely
        claims = []Claim{}
    }

    // Determine reliability tier
    tier := fc.determineReliabilityTier(score)

    result := &FactCheckResult{
        Score:          score,
        ReliabilityTier: tier,
        Claims:         claims,
        Timestamp:      time.Now(),
    }

    // Cache the result
    fc.cacheResult(article.URL, result)

    return result, nil
}

// analyzeReliability calculates a reliability score for an article
func (fc *FactChecker) analyzeReliability(ctx context.Context, article *NewsArticle) (float64, error) {
    // Initialize base score
    score := 0.5

    // Check source reliability
    sourceScore := fc.getSourceReliabilityScore(article.Source)
    score = (score + sourceScore) / 2

    // Check content indicators
    contentScore := fc.analyzeContent(article.Content)
    score = (score + contentScore) / 2

    // Check external citations
    if len(article.Citations) > 0 {
        citationScore := fc.analyzeCitations(article.Citations)
        score = (score + citationScore) / 2
    }

    // Ensure score is within bounds
    if score < 0 {
        score = 0
    }
    if score > 1 {
        score = 1
    }

    return score, nil
}

// extractClaims identifies and verifies major claims in the article
func (fc *FactChecker) extractClaims(ctx context.Context, article *NewsArticle) ([]Claim, error) {
    var claims []Claim

    // Extract potential claims from content
    sentences := strings.Split(article.Content, ".")
    for _, sentence := range sentences {
        if fc.isClaim(sentence) {
            claim := Claim{
                Text: strings.TrimSpace(sentence),
            }

            // Verify the claim
            rating, evidence := fc.verifyClaim(ctx, claim.Text)
            claim.Rating = rating
            claim.Evidence = evidence

            claims = append(claims, claim)
        }
    }

    return claims, nil
}

// determineReliabilityTier converts a numerical score to a tier
func (fc *FactChecker) determineReliabilityTier(score float64) string {
    switch {
    case score >= 0.8:
        return "High"
    case score >= 0.5:
        return "Medium"
    default:
        return "Low"
    }
}

// Cache management methods

func (fc *FactChecker) getCachedResult(url string) *FactCheckResult {
    fc.cacheMu.RLock()
    defer fc.cacheMu.RUnlock()

    if result, exists := fc.cache[url]; exists {
        if time.Since(result.Timestamp) < fc.cacheTime {
            return result
        }
        // Expired cache entry
        delete(fc.cache, url)
    }
    return nil
}

func (fc *FactChecker) cacheResult(url string, result *FactCheckResult) {
    fc.cacheMu.Lock()
    defer fc.cacheMu.Unlock()
    fc.cache[url] = result
}

// Helper methods

func (fc *FactChecker) getSourceReliabilityScore(source string) float64 {
    // Implementation would include checking against a database of known reliable sources
    // For now, return a neutral score
    return 0.5
}

func (fc *FactChecker) analyzeContent(content string) float64 {
    score := 0.5

    // Check for clickbait indicators
    if fc.hasClickbaitPatterns(content) {
        score -= 0.2
    }

    // Check for emotional manipulation
    if fc.hasEmotionalManipulation(content) {
        score -= 0.1
    }

    // Check for balanced reporting
    if fc.hasBalancedPerspectives(content) {
        score += 0.2
    }

    return score
}

func (fc *FactChecker) analyzeCitations(citations []string) float64 {
    if len(citations) == 0 {
        return 0.3
    }

    reliableCount := 0
    for _, citation := range citations {
        if fc.isReliableCitation(citation) {
            reliableCount++
        }
    }

    return float64(reliableCount) / float64(len(citations))
}

func (fc *FactChecker) isClaim(sentence string) bool {
    // Simple heuristic for identifying claims
    claimIndicators := []string{
        "according to",
        "researchers found",
        "studies show",
        "evidence suggests",
        "experts say",
    }

    sentence = strings.ToLower(strings.TrimSpace(sentence))
    for _, indicator := range claimIndicators {
        if strings.Contains(sentence, indicator) {
            return true
        }
    }

    return false
}

func (fc *FactChecker) verifyClaim(ctx context.Context, claim string) (string, string) {
    // This would integrate with external fact-checking APIs
    // For now, return a neutral rating
    return "Unverified", "No verification data available"
}

func (fc *FactChecker) hasClickbaitPatterns(content string) bool {
    patterns := []string{
        "you won't believe",
        "shocking",
        "mind-blowing",
        "doctors hate",
        "this one weird trick",
    }

    content = strings.ToLower(content)
    for _, pattern := range patterns {
        if strings.Contains(content, pattern) {
            return true
        }
    }
    return false
}

func (fc *FactChecker) hasEmotionalManipulation(content string) bool {
    patterns := []string{
        "must see",
        "warning",
        "alarming",
        "terrifying",
        "outrageous",
    }

    content = strings.ToLower(content)
    for _, pattern := range patterns {
        if strings.Contains(content, pattern) {
            return true
        }
    }
    return false
}

func (fc *FactChecker) hasBalancedPerspectives(content string) bool {
    indicators := []string{
        "however",
        "on the other hand",
        "alternatively",
        "in contrast",
        "different perspective",
    }

    content = strings.ToLower(content)
    for _, indicator := range indicators {
        if strings.Contains(content, indicator) {
            return true
        }
    }
    return false
}

func (fc *FactChecker) isReliableCitation(citation string) bool {
    // This would check against a database of reliable sources
    // For now, implement basic checks
    reliableDomains := []string{
        ".edu",
        ".gov",
        "nature.com",
        "science.org",
        "reuters.com",
    }

    citation = strings.ToLower(citation)
    for _, domain := range reliableDomains {
        if strings.Contains(citation, domain) {
            return true
        }
    }
    return false
}
