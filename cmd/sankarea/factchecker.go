// cmd/sankarea/factchecker.go
package main

import (
    "context"
    "fmt"
    "net/http"
    "regexp"
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
    Score           float64   `json:"score"`
    ReliabilityTier string    `json:"reliability_tier"`
    Claims          []Claim   `json:"claims,omitempty"`
    Reasons         []string  `json:"reasons,omitempty"` // Added from factcheck.go
    Timestamp       time.Time `json:"timestamp"`
}

// Claim represents a verified claim in an article
type Claim struct {
    Text     string `json:"text"`
    Rating   string `json:"rating"`
    Evidence string `json:"evidence,omitempty"`
}

const (
    // Reliability tiers
    TierHigh   = "High"
    TierMedium = "Medium"
    TierLow    = "Low"

    // Score thresholds
    ScoreHigh   = 0.8
    ScoreMedium = 0.5
)

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
    score, reasons, err := fc.analyzeReliability(ctx, article)
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
        Score:           score,
        ReliabilityTier: tier,
        Claims:          claims,
        Reasons:         reasons,
        Timestamp:       time.Now(),
    }

    // Cache the result
    fc.cacheResult(article.URL, result)

    return result, nil
}

// analyzeReliability calculates a reliability score for an article
func (fc *FactChecker) analyzeReliability(ctx context.Context, article *NewsArticle) (float64, []string, error) {
    var reasons []string
    score := 0.5

    // Check source reliability
    sourceScore, sourceReason := fc.getSourceReliabilityScore(article.Source)
    if sourceReason != "" {
        reasons = append(reasons, sourceReason)
    }
    score = (score + sourceScore) / 2

    // Check content indicators
    contentScore, contentReasons := fc.analyzeContent(article.Content)
    reasons = append(reasons, contentReasons...)
    score = (score + contentScore) / 2

    // Check external citations
    if len(article.Citations) > 0 {
        citationScore, citationReason := fc.analyzeCitations(article.Citations)
        if citationReason != "" {
            reasons = append(reasons, citationReason)
        }
        score = (score + citationScore) / 2
    }

    // Ensure score is within bounds
    if score < 0 {
        score = 0
    }
    if score > 1 {
        score = 1
    }

    return score, reasons, nil
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
    case score >= ScoreHigh:
        return TierHigh
    case score >= ScoreMedium:
        return TierMedium
    default:
        return TierLow
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

func (fc *FactChecker) getSourceReliabilityScore(source string) (float64, string) {
    reliableSources := map[string]bool{
        "Reuters":     true,
        "AP News":     true,
        "BBC News":    true,
        "Tech News":   true,
        "World News":  true,
        "Bloomberg":   true,
        "Nature":      true,
        "Science":     true,
    }

    if reliableSources[source] {
        return 1.0, fmt.Sprintf("Source '%s' is known to be reliable", source)
    }

    return 0.5, fmt.Sprintf("Source '%s' reliability is uncertain", source)
}

func (fc *FactChecker) analyzeContent(content string) (float64, []string) {
    score := 0.5
    var reasons []string

    // Check for clickbait indicators
    if hasClickbait := fc.hasClickbaitPatterns(content); hasClickbait {
        score -= 0.2
        reasons = append(reasons, "Contains clickbait patterns")
    }

    // Check for emotional manipulation
    if hasEmotional := fc.hasEmotionalManipulation(content); hasEmotional {
        score -= 0.1
        reasons = append(reasons, "Contains emotional manipulation")
    }

    // Check for balanced reporting
    if hasBalanced := fc.hasBalancedPerspectives(content); hasBalanced {
        score += 0.2
        reasons = append(reasons, "Shows balanced perspectives")
    }

    // Check for excessive punctuation
    if hasExcessive := fc.hasExcessivePunctuation(content); hasExcessive {
        score -= 0.1
        reasons = append(reasons, "Contains excessive punctuation")
    }

    return score, reasons
}

func (fc *FactChecker) analyzeCitations(citations []string) (float64, string) {
    if len(citations) == 0 {
        return 0.3, "No citations provided"
    }

    reliableCount := 0
    for _, citation := range citations {
        if fc.isReliableCitation(citation) {
            reliableCount++
        }
    }

    score := float64(reliableCount) / float64(len(citations))
    
    switch {
    case reliableCount >= 3:
        return score, "Multiple reliable citations provided"
    case reliableCount >= 1:
        return score, "At least one reliable citation provided"
    default:
        return score, "Limited reliable citations"
    }
}

func (fc *FactChecker) isClaim(sentence string) bool {
    claimIndicators := []string{
        "according to",
        "researchers found",
        "studies show",
        "evidence suggests",
        "experts say",
        "research indicates",
        "data shows",
        "analysis reveals",
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
        "secret they don't want you to know",
        "miracle cure",
        "instant results",
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
        "devastating",
        "life-changing",
        "revolutionary",
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
        "some argue",
        "others suggest",
        "contrary to",
    }

    content = strings.ToLower(content)
    count := 0
    for _, indicator := range indicators {
        if strings.Contains(content, indicator) {
            count++
        }
    }
    return count >= 2
}

func (fc *FactChecker) hasExcessivePunctuation(content string) bool {
    patterns := []string{
        `[!]{2,}`,
        `[?]{2,}`,
        `[.]{4,}`,
    }

    for _, pattern := range patterns {
        if matched, _ := regexp.MatchString(pattern, content); matched {
            return true
        }
    }

    return false
}

func (fc *FactChecker) isReliableCitation(citation string) bool {
    reliableDomains := []string{
        ".edu",
        ".gov",
        "nature.com",
        "science.org",
        "reuters.com",
        "ap.org",
        "bbc.com",
        "bloomberg.com",
    }

    citation = strings.ToLower(citation)
    for _, domain := range reliableDomains {
        if strings.Contains(citation, domain) {
            return true
        }
    }
    return false
}
