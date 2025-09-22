package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
	"io"
	"net/http"
	"net/url"

	"github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/api/bsky"
	"github.com/bluesky-social/indigo/lex/util"
	"github.com/bluesky-social/indigo/xrpc"
	"golang.org/x/term"
)

// Config holds our application configuration
type Config struct {
	Handle      string
	AppPassword string
	ListURI     string
}

// UserData represents the structure of processed_haters.json
type UserData map[string]interface{}

// RetryConfig holds retry configuration
type RetryConfig struct {
	MaxRetries int
	BaseWait   time.Duration
}

// BlueskyBlocklistManager manages blocklist operations
type BlueskyBlocklistManager struct {
	client      util.LexClient
	config      Config
	retryConfig RetryConfig
}

// NewBlueskyBlocklistManager creates a new manager instance
func NewBlueskyBlocklistManager() *BlueskyBlocklistManager {
	return &BlueskyBlocklistManager{
		retryConfig: RetryConfig{
			MaxRetries: 5,
			BaseWait:   60 * time.Second,
		},
	}
}

// getCredentials prompts user for credentials
func (m *BlueskyBlocklistManager) getCredentials() error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Enter your Bluesky handle (e.g., username.bsky.social): ")
	handle, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read handle: %w", err)
	}
	m.config.Handle = strings.TrimSpace(handle)

	fmt.Print("Enter your app password: ")
	passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return fmt.Errorf("failed to read password: %w", err)
	}
	fmt.Println() // New line after password input
	m.config.AppPassword = strings.TrimSpace(string(passwordBytes))

	fmt.Print("Enter the list URI or AT-URI (starts with at://): ")
	listURI, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read list URI: %w", err)
	}
	m.config.ListURI = strings.TrimSpace(listURI)

	if !strings.HasPrefix(m.config.ListURI, "at://") {
		return fmt.Errorf("list identifier should be an AT-URI starting with 'at://'")
	}

	return nil
}

// authenticate logs in to Bluesky using indigo's xrpc client
// authenticate logs in to Bluesky using indigo's xrpc client
func (m *BlueskyBlocklistManager) authenticate() error {
    // Create xrpc client for bsky.social
    xrpcClient := &xrpc.Client{
        Host: "https://pds.futur.blue",
    }
    
    fmt.Printf("DEBUG: Connecting to PDS at: %s\n", xrpcClient.Host)

    // Perform authentication using com.atproto.server.createSession
    auth := &atproto.ServerCreateSession_Input{
        Identifier: m.config.Handle,
        Password:   m.config.AppPassword,
    }
    
    fmt.Printf("DEBUG: Authenticating with handle: %s\n", m.config.Handle)

    ctx := context.Background()
    session, err := atproto.ServerCreateSession(ctx, xrpcClient, auth)
    if err != nil {
        return fmt.Errorf("failed to create session: %w", err)
    }

    // Set authentication token for future requests
    xrpcClient.Auth = &xrpc.AuthInfo{
        AccessJwt:  session.AccessJwt,
        RefreshJwt: session.RefreshJwt,
        Handle:     session.Handle,
        Did:        session.Did,
    }
    
    fmt.Printf("DEBUG: Successfully authenticated. User DID: %s\n", session.Did)

    // Use the xrpc client as a LexClient
    m.client = xrpcClient
    return nil
}

// getUserDIDs reads DIDs from the JSON file
func (m *BlueskyBlocklistManager) getUserDIDs(filename string) ([]string, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filename, err)
	}

	var userData UserData
	if err := json.Unmarshal(data, &userData); err != nil {
		return nil, fmt.Errorf("failed to parse JSON from %s: %w", filename, err)
	}

	var dids []string
	for did := range userData {
		dids = append(dids, did)
	}

	return dids, nil
}


// fetchBlocklistDIDs fetches all DIDs from an existing blocklist using indigo.
func (m *BlueskyBlocklistManager) fetchBlocklistDIDs(listURI string) ([]string, error) {
	// Scoped types
	type Subject struct {
		DID string `json:"did"`
	}

	type Item struct {
		Subject Subject `json:"subject"`
	}

	type Response struct {
		Cursor string `json:"cursor"`
		Items  []Item `json:"items"`
	}

	baseURL := "https://public.api.bsky.app/xrpc/app.bsky.graph.getList"
	var dids []string
	cursor := ""

	for {
		// Build request URL
		params := url.Values{}
		params.Add("list", listURI)
		if cursor != "" {
			params.Add("cursor", cursor)
		}
		reqURL := baseURL + "?" + params.Encode()

		// Make the GET request
		resp, err := http.Get(reqURL)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("non-200 response: %d", resp.StatusCode)
		}

		// Parse the response
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		var parsed Response
		if err := json.Unmarshal(body, &parsed); err != nil {
			return nil, err
		}

		// Extract DIDs
		for _, item := range parsed.Items {
			dids = append(dids, item.Subject.DID)
		}

		// Stop if there's no next page
		if parsed.Cursor == "" {
			break
		}
		cursor = parsed.Cursor
	}

	return dids, nil
}


// extractRateLimitHeaders extracts rate limit info from error
func (m *BlueskyBlocklistManager) extractRateLimitHeaders(err error) (resetTime int64, remaining int) {
	errStr := err.Error()
	
	// Try to extract rate limit reset time from error message
	resetPattern := regexp.MustCompile(`ratelimit-reset['"']?\s*:\s*['"]?(\d+)['"]?`)
	remainingPattern := regexp.MustCompile(`ratelimit-remaining['"']?\s*:\s*['"]?(\d+)['"]?`)
	
	if matches := resetPattern.FindStringSubmatch(errStr); len(matches) > 1 {
		if rt, parseErr := strconv.ParseInt(matches[1], 10, 64); parseErr == nil {
			resetTime = rt
		}
	}
	
	if matches := remainingPattern.FindStringSubmatch(errStr); len(matches) > 1 {
		if rem, parseErr := strconv.Atoi(matches[1]); parseErr == nil {
			remaining = rem
		}
	}
	
	return resetTime, remaining
}

// calculateWaitTime determines how long to wait before retrying
func (m *BlueskyBlocklistManager) calculateWaitTime(err error, attempt int) time.Duration {
	// First retry: try to use rate limit reset time
	if attempt == 1 {
		resetTime, remaining := m.extractRateLimitHeaders(err)
		
		if resetTime > 0 {
			currentTime := time.Now().Unix()
			waitUntilReset := time.Duration(resetTime-currentTime+1) * time.Second
			
			if waitUntilReset > 0 && waitUntilReset < 48*time.Hour {
				fmt.Printf("Rate limit exceeded (remaining: %d), waiting until reset + 1s\n", remaining)
				return waitUntilReset
			}
		}
	}
	
	// Exponential backoff: 60 * (2 ^ (attempt - 1))
	exponentialWait := time.Duration(60*math.Pow(2, float64(attempt-1))) * time.Second
	fmt.Printf("Using exponential backoff for attempt #%d: %v\n", attempt, exponentialWait)
	return exponentialWait
}

// createListItemWithRetry adds a user to the list with retry logic using indigo
func (m *BlueskyBlocklistManager) createListItemWithRetry(userDID, listURI string) error {
	ctx := context.Background()

	for attempt := 1; attempt <= m.retryConfig.MaxRetries; attempt++ {
		// Create the list item using the proper bsky type
		record := &bsky.GraphListitem{
			Subject:   userDID,
			List:      listURI,
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		}

		// Get the authenticated DID from the client
		xrpcClient := m.client.(*xrpc.Client)
		if xrpcClient.Auth == nil {
			return fmt.Errorf("not authenticated")
		}

		createInput := &atproto.RepoCreateRecord_Input{
			Repo:       xrpcClient.Auth.Did,
			Collection: "app.bsky.graph.listitem",
			Record:     &util.LexiconTypeDecoder{Val: record}, // Wrap in LexiconTypeDecoder
		}

		_, err := atproto.RepoCreateRecord(ctx, m.client, createInput)
		if err == nil {
			return nil // Success
		}

		if attempt == m.retryConfig.MaxRetries {
			return fmt.Errorf("final attempt failed after %d tries: %w", m.retryConfig.MaxRetries, err)
		}

		waitTime := m.calculateWaitTime(err, attempt)
		fmt.Printf("Attempt %d/%d failed: %v\n", attempt, m.retryConfig.MaxRetries, err)
		fmt.Printf("Waiting %v before retry...\n", waitTime)
		time.Sleep(waitTime)
	}

	return fmt.Errorf("maximum retries exceeded")
}

// removeDuplicates removes duplicate DIDs from a slice
func removeDuplicates(slice []string) []string {
	keys := make(map[string]bool)
	var result []string

	for _, item := range slice {
		if !keys[item] {
			keys[item] = true
			result = append(result, item)
		}
	}

	return result
}

// difference returns elements in a that are not in b
func difference(a, b []string) []string {
	mb := make(map[string]bool, len(b))
	for _, x := range b {
		mb[x] = true
	}

	var result []string
	for _, x := range a {
		if !mb[x] {
			result = append(result, x)
		}
	}

	return result
}

// run executes the main application logic
func (m *BlueskyBlocklistManager) run() error {
	fmt.Println("Bluesky Blocklist Manager")
	fmt.Println("=" + strings.Repeat("=", 24))

	// Get credentials
	if err := m.getCredentials(); err != nil {
		return fmt.Errorf("failed to get credentials: %w", err)
	}

	// Get DIDs to add
	userDIDs, err := m.getUserDIDs("processed_haters.json")
	if err != nil {
		return fmt.Errorf("failed to get user DIDs: %w", err)
	}

	if len(userDIDs) == 0 {
		return fmt.Errorf("no valid DIDs provided")
	}

	fmt.Printf("\nFound %d DIDs to add to the list.\n", len(userDIDs))

	// Authenticate
	fmt.Println("\nConnecting to Bluesky...")
	if err := m.authenticate(); err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}
	fmt.Println("✓ Successfully authenticated")

	// Fetch existing DIDs from the blocklist
	fmt.Println("Fetching existing blocklist entries...")
	alreadyListed, err := m.fetchBlocklistDIDs(m.config.ListURI)
	if err != nil {
		return fmt.Errorf("failed to fetch existing blocklist: %w", err)
	}

	fmt.Printf("List already contains %d DIDs\n", len(alreadyListed))

	// Remove duplicates and filter out already listed DIDs
	userDIDs = difference(removeDuplicates(userDIDs), alreadyListed)
	fmt.Printf("DIDs to be added to list: %d\n", len(userDIDs))

	if len(userDIDs) == 0 {
		fmt.Println("All DIDs are already in the list. Nothing to do.")
		return nil
	}

	// Confirm before proceeding
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Add %d users to list %s? (y/N): ", len(userDIDs), m.config.ListURI)
	confirm, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read confirmation: %w", err)
	}

	if strings.TrimSpace(strings.ToLower(confirm)) != "y" {
		fmt.Println("Operation cancelled.")
		return nil
	}

	// Add users to the list
	successful := 0
	failed := 0

	for i, userDID := range userDIDs {
		fmt.Printf("Adding user %d/%d: %s\n", i+1, len(userDIDs), userDID)
		if err := m.createListItemWithRetry(userDID, m.config.ListURI); err != nil {
			fmt.Printf("✗ Failed to add %s: %v\n", userDID, err)
			failed++
		} else {
			fmt.Println("✓ Added successfully")
			successful++
		}
	}

	// Summary
	fmt.Println("\nOperation complete!")
	fmt.Printf("Successfully added: %d\n", successful)
	fmt.Printf("Failed: %d\n", failed)

	if successful > 0 {
		fmt.Printf("\n✓ %d users have been added to your blocklist.\n", successful)
	}

	return nil
}

func main() {
	manager := NewBlueskyBlocklistManager()
	if err := manager.run(); err != nil {
		log.Fatalf("Error: %v", err)
	}
}