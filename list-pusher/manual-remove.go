package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/api/bsky"
	"github.com/bluesky-social/indigo/xrpc"
)

// Config holds our application configuration
type Config struct {
	Handle      string
	AppPassword string
	ListURI     string
}

// TOMLConfig represents the structure of the TOML file
type TOMLConfig struct {
	Removes struct {
		Identifiers []string `toml:"identifiers"`
	} `toml:"Removes"`
	Adds struct {
		Identifiers []string `toml:"identifiers"`
	} `toml:"Adds"`
}

// BlueskyListRemover manages list removal operations
type BlueskyListRemover struct {
	client *xrpc.Client
	config Config
}

// NewBlueskyListRemover creates a new remover instance
func NewBlueskyListRemover() *BlueskyListRemover {
	return &BlueskyListRemover{}
}

// loadConfig loads configuration from environment variables
func (r *BlueskyListRemover) loadConfig() error {
	r.config.Handle = os.Getenv("BLUESKY_HANDLE")
	r.config.AppPassword = os.Getenv("BLUESKY_APP_PASSWORD")
	r.config.ListURI = os.Getenv("BLUESKY_LIST_URI")

	// Validate required fields
	if r.config.Handle == "" {
		return fmt.Errorf("BLUESKY_HANDLE environment variable is required")
	}
	if r.config.AppPassword == "" {
		return fmt.Errorf("BLUESKY_APP_PASSWORD environment variable is required")
	}
	if r.config.ListURI == "" {
		return fmt.Errorf("BLUESKY_LIST_URI environment variable is required")
	}

	if !strings.HasPrefix(r.config.ListURI, "at://") {
		return fmt.Errorf("BLUESKY_LIST_URI should be an AT-URI starting with 'at://'")
	}

	return nil
}

// authenticate logs in to Bluesky
func (r *BlueskyListRemover) authenticate() error {
	// Create xrpc client for bsky.social
	r.client = &xrpc.Client{
		Host: "https://pds.futur.blue",
	}

	fmt.Printf("DEBUG: Connecting to PDS at: %s\n", r.client.Host)

	// Perform authentication
	auth := &atproto.ServerCreateSession_Input{
		Identifier: r.config.Handle,
		Password:   r.config.AppPassword,
	}

	fmt.Printf("DEBUG: Authenticating with handle: %s\n", r.config.Handle)

	ctx := context.Background()
	session, err := atproto.ServerCreateSession(ctx, r.client, auth)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	// Set authentication token for future requests
	r.client.Auth = &xrpc.AuthInfo{
		AccessJwt:  session.AccessJwt,
		RefreshJwt: session.RefreshJwt,
		Handle:     session.Handle,
		Did:        session.Did,
	}

	fmt.Printf("DEBUG: Successfully authenticated. User DID: %s\n", session.Did)
	return nil
}

// readRemovalsFromTOML reads the Removes section from the TOML file
func (r *BlueskyListRemover) readRemovalsFromTOML(filename string) ([]string, error) {
	var config TOMLConfig

	if _, err := toml.DecodeFile(filename, &config); err != nil {
		return nil, fmt.Errorf("failed to parse TOML file %s: %w", filename, err)
	}

	if len(config.Removes.Identifiers) == 0 {
		return nil, fmt.Errorf("no identifiers found in [Removes] section")
	}

	return config.Removes.Identifiers, nil
}

// resolveHandleToDID resolves a handle to a DID using com.atproto.identity.resolveHandle
func (r *BlueskyListRemover) resolveHandleToDID(handle string) (string, error) {
	ctx := context.Background()

	resp, err := atproto.IdentityResolveHandle(ctx, r.client, handle)
	if err != nil {
		return "", fmt.Errorf("failed to resolve handle %s: %w", handle, err)
	}

	return resp.Did, nil
}

// ListItem represents a list item with its record key
type ListItem struct {
	DID       string
	RecordKey string
}

// fetchListItems fetches all list items and their record keys for removal
func (r *BlueskyListRemover) fetchListItems(listURI string) ([]ListItem, error) {
	var items []ListItem
	cursor := ""
	limit := int64(100)

	for {
		ctx := context.Background()

		resp, err := bsky.GraphGetList(ctx, r.client, cursor, limit, listURI)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch list: %w", err)
		}

		// Extract DIDs and record keys from the response
		for _, item := range resp.Items {
			if item.Subject != nil && item.Subject.Did != "" && item.Uri != "" {
				// Extract record key from URI (at://did/collection/recordkey)
				parts := strings.Split(item.Uri, "/")
				if len(parts) >= 4 {
					recordKey := parts[len(parts)-1]
					items = append(items, ListItem{
						DID:       item.Subject.Did,
						RecordKey: recordKey,
					})
				}
			}
		}

		// Stop if there's no next page
		if resp.Cursor == nil || *resp.Cursor == "" {
			break
		}
		cursor = *resp.Cursor

		time.Sleep(100 * time.Millisecond)
	}

	return items, nil
}

// removeListItem removes a specific list item by its record key
func (r *BlueskyListRemover) removeListItem(recordKey string) error {
	ctx := context.Background()

	deleteInput := &atproto.RepoDeleteRecord_Input{
		Repo:       r.client.Auth.Did,
		Collection: "app.bsky.graph.listitem",
		Rkey:       recordKey,
	}

	_, err := atproto.RepoDeleteRecord(ctx, r.client, deleteInput)
	if err != nil {
		return fmt.Errorf("failed to delete record %s: %w", recordKey, err)
	}

	return nil
}

// run executes the main application logic
func (r *BlueskyListRemover) run() error {
	fmt.Println("Bluesky List Remover")
	fmt.Println("=" + strings.Repeat("=", 19))

	// Load config from environment
	if err := r.loadConfig(); err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	fmt.Printf("Using handle: %s\n", r.config.Handle)
	fmt.Printf("Using list: %s\n", r.config.ListURI)

	// Read handles from TOML file
	handles, err := r.readRemovalsFromTOML("manual-changes.toml")
	if err != nil {
		return fmt.Errorf("failed to read TOML file: %w", err)
	}

	if len(handles) == 0 {
		return fmt.Errorf("no valid identifiers provided in [Removes] section")
	}

	fmt.Printf("\nFound %d identifiers to remove from the list.\n", len(handles))

	// Authenticate
	fmt.Println("\nConnecting to Bluesky...")
	if err := r.authenticate(); err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}
	fmt.Println("✓ Successfully authenticated")

	// Resolve handles to DIDs
	fmt.Println("\nResolving handles to DIDs...")
	var userDIDs []string
	for i, handle := range handles {
		fmt.Printf("Resolving %d/%d: %s\n", i+1, len(handles), handle)
		did, err := r.resolveHandleToDID(handle)
		if err != nil {
			fmt.Printf("✗ Failed to resolve %s: %v (skipping)\n", handle, err)
			continue
		}
		userDIDs = append(userDIDs, did)
		fmt.Printf("  → %s\n", did)
		time.Sleep(100 * time.Millisecond) // Rate limit friendly
	}

	if len(userDIDs) == 0 {
		return fmt.Errorf("no handles could be resolved to DIDs")
	}

	fmt.Printf("\nSuccessfully resolved %d/%d handles\n", len(userDIDs), len(handles))

	// Fetch all list items once to get record keys
	fmt.Println("\nFetching list items...")
	listItems, err := r.fetchListItems(r.config.ListURI)
	if err != nil {
		return fmt.Errorf("failed to fetch list items: %w", err)
	}

	fmt.Printf("Found %d total items in list\n", len(listItems))

	// Create a map for quick lookup
	didToRecordKey := make(map[string]string)
	for _, item := range listItems {
		didToRecordKey[item.DID] = item.RecordKey
	}

	// Find which DIDs are actually in the list
	var toRemove []ListItem
	for _, did := range userDIDs {
		if recordKey, exists := didToRecordKey[did]; exists {
			toRemove = append(toRemove, ListItem{DID: did, RecordKey: recordKey})
		}
	}

	fmt.Printf("DIDs found in list to remove: %d\n", len(toRemove))

	if len(toRemove) == 0 {
		fmt.Println("None of the specified DIDs are in the list. Nothing to do.")
		return nil
	}

	// Confirm before proceeding
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("\nRemove %d users from list %s? (y/N): ", len(toRemove), r.config.ListURI)
	confirm, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read confirmation: %w", err)
	}

	if strings.TrimSpace(strings.ToLower(confirm)) != "y" {
		fmt.Println("Operation cancelled.")
		return nil
	}

	// Remove users from the list
	successful := 0
	failed := 0

	for i, item := range toRemove {
		fmt.Printf("Removing user %d/%d: %s\n", i+1, len(toRemove), item.DID)
		if err := r.removeListItem(item.RecordKey); err != nil {
			fmt.Printf("✗ Failed to remove %s: %v\n", item.DID, err)
			failed++
		} else {
			fmt.Println("✓ Removed successfully")
			successful++
		}

		// Small delay to avoid rate limiting
		time.Sleep(100 * time.Millisecond)
	}

	// Summary
	fmt.Println("\nOperation complete!")
	fmt.Printf("Successfully removed: %d\n", successful)
	fmt.Printf("Failed: %d\n", failed)

	if successful > 0 {
		fmt.Printf("\n✓ %d users have been removed from your list.\n", successful)
	}

	return nil
}

func main() {
	remover := NewBlueskyListRemover()
	if err := remover.run(); err != nil {
		log.Fatalf("Error: %v", err)
	}
}
