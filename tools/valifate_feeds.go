// Current Date and Time (UTC): 2025-05-22 15:14:41
// Current User's Login: NullMeDev
// File name: tools/validate_feeds.go

package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v2"
)

// Source represents an RSS feed source (simplified for validation)
type Source struct {
	Name    string `yaml:"name"`
	URL     string `yaml:"url"`
	Active  bool   `yaml:"active"`
	Paused  bool   `yaml:"paused"`
}

func main() {
	fmt.Println("RSS Feed Validator")
	fmt.Println("=================")
	fmt.Println("Validating all feeds in sources.yml")
	
	// Read sources file
	data, err := ioutil.ReadFile("config/sources.yml")
	if err != nil {
		fmt.Printf("Error reading sources file: %v\n", err)
		os.Exit(1)
	}

	var sources []Source
	if err := yaml.Unmarshal(data, &sources); err != nil {
		fmt.Printf("Error parsing sources file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Found %d sources to validate\n\n", len(sources))

	// Set up HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Setup result channels
	type Result struct {
		Source  Source
		Valid   bool
		Message string
		Time    time.Duration
	}
	results := make(chan Result, len(sources))
	
	// Use a wait group to wait for all goroutines to finish
	var wg sync.WaitGroup
	
	// Test each feed
	for _, src := range sources {
		wg.Add(1)
		go func(src Source) {
			defer wg.Done()
			
			if !src.Active || src.Paused {
				results <- Result{
					Source:  src,
					Valid:   true,
					Message: "SKIPPED (inactive)",
					Time:    0,
				}
				return
			}
			
			start := time.Now()
			req, err := http.NewRequest("GET", src.URL, nil)
			if err != nil {
				results <- Result{
					Source:  src,
					Valid:   false,
					Message: fmt.Sprintf("Invalid URL: %v", err),
					Time:    time.Since(start),
				}
				return
			}
			
			// Set a user agent
			req.Header.Set("User-Agent", "Sankarea Feed Validator/1.0")
			
			resp, err := client.Do(req)
			if err != nil {
				results <- Result{
					Source:  src,
					Valid:   false,
					Message: fmt.Sprintf("Request failed: %v", err),
					Time:    time.Since(start),
				}
				return
			}
			defer resp.Body.Close()
			
			if resp.StatusCode != http.StatusOK {
				results <- Result{
					Source:  src,
					Valid:   false,
					Message: fmt.Sprintf("HTTP error: %s", resp.Status),
					Time:    time.Since(start),
				}
				return
			}
			
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				results <- Result{
					Source:  src,
					Valid:   false,
					Message: fmt.Sprintf("Failed to read response: %v", err),
					Time:    time.Since(start),
				}
				return
			}
			
			// Simple check for RSS/XML format
			content := string(body)
			if !(strings.Contains(content, "<rss") || 
			     strings.Contains(content, "<feed") || 
				 strings.Contains(content, "<xml")) {
				results <- Result{
					Source:  src,
					Valid:   false,
					Message: "Response doesn't appear to be RSS/XML",
					Time:    time.Since(start),
				}
				return
			}
			
			results <- Result{
				Source:  src,
				Valid:   true,
				Message: "OK",
				Time:    time.Since(start),
			}
		}(src)
	}
	
	// Close results channel when all goroutines are done
	go func() {
		wg.Wait()
		close(results)
	}()
	
	// Process results
	var valid, invalid int
	invalidSources := make([]string, 0)
	
	for result := range results {
		if result.Valid && result.Message != "SKIPPED (inactive)" {
			fmt.Printf("✅ %-30s [%7dms] %s\n", 
				result.Source.Name, 
				result.Time.Milliseconds(),
				result.Message)
			valid++
		} else if result.Message == "SKIPPED (inactive)" {
			fmt.Printf("⏭️  %-30s %s\n", 
				result.Source.Name, 
				result.Message)
		} else {
			fmt.Printf("❌ %-30s [%7dms] %s\n", 
				result.Source.Name, 
				result.Time.Milliseconds(),
				result.Message)
			invalid++
			invalidSources = append(invalidSources, result.Source.Name)
		}
	}
	
	// Print summary
	fmt.Println("\nValidation Summary:")
	fmt.Printf("Valid feeds:   %d\n", valid)
	fmt.Printf("Invalid feeds: %d\n", invalid)
	
	if invalid > 0 {
		fmt.Println("\nInvalid sources:")
		for _, name := range invalidSources {
			fmt.Printf("- %s\n", name)
		}
		os.Exit(1)
	}
}
