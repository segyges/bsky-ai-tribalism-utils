#!/usr/bin/env python3
"""
Bluesky Blocklist CLI Script
Adds a list of user DIDs to an existing blocklist on Bluesky
"""

import sys
import getpass
import json
from typing import List
from atproto import Client
from atproto_client.models.app.bsky.graph.get_list import Params as GetListParams
import time
import backoff
from datetime import datetime

def fetch_blocklist_dids(client: Client, at_uri: str) -> List[str]:
    """
    Fetch all DIDs from a Bluesky blocklist given its AT-URI.
    
    Args:
        client: Already authenticated Bluesky client
        at_uri: The AT-URI of the blocklist (e.g., "at://did:plc:s45hbf5dqdkjpwuuq4djo6l2/app.bsky.graph.list/3lzefe5k2432n")
    
    Returns:
        List of DIDs as strings
    """
    all_dids = []
    cursor = None
    
    while True:
        # Create params object for the API call
        if cursor:
            params = GetListParams(
                list=at_uri,
                cursor=cursor
            )
        else:
            params = GetListParams(
                list=at_uri,
            )
        
        # Fetch a page of list members
        response = client.app.bsky.graph.get_list(params)
        
        # Extract DIDs from this page
        if hasattr(response, 'items') and response.items:
            page_dids = []
            for member in response.items:
                subject = getattr(member, 'subject', None)
                if subject:
                    did = getattr(subject, 'did', None)
                    if did:
                        page_dids.append(did)
            
            all_dids.extend(page_dids)
        
        # Check if there are more pages
        if hasattr(response, 'cursor') and response.cursor:
            cursor = response.cursor
        else:
            break
    
    return all_dids

def get_user_dids(filename="processed_haters.json") -> List[str]:
    """
    Get the list of user DIDs to add to the blocklist from processed_haters.json.
    """
    try:
        with open(filename, 'r') as f:
            data = json.load(f)
            return list(data.keys())
    except FileNotFoundError:
        print(f"Error: File not found: {filename}")
        return []
    except json.JSONDecodeError:
        print(f"Error: Invalid JSON in file: {filename}")
        return []


def log_backoff(details):
    print(
        f"Retrying ({details['tries']}/5) after exception: {details['exception']} "
        f"(waiting {details['wait']:0.1f}s)"
    )
    
def smart_backoff(details):
    # First retry and we have rate limit info? Wait until reset + 1 second
    if details['tries'] == 1 and hasattr(details.get('exception'), 'headers'):
        reset_time = details['exception'].headers.get('ratelimit-reset')
        if reset_time:
            current_time = time.time()
            wait_time = int(reset_time) - current_time + 1
            if wait_time > 0 and wait_time < 3600:  # reasonable bounds
                print(f"Using rate limit reset time: {reset_time} (waiting {wait_time:.1f}s until reset + 1s)")
                return wait_time
            else:
                print(f"Rate limit reset time unreasonable ({wait_time:.1f}s), falling back to exponential")
        else:
            print("No ratelimit-reset header found, falling back to exponential")
    
    # Fall back to exponential: 60 * (2 ^ (tries - 1))
    exp_wait = 60 * (2 ** (details['tries'] - 1))
    print(f"Using exponential backoff: {exp_wait:.1f}s (try #{details['tries']})")
    return exp_wait
@backoff.on_exception(
    smart_backoff,  # Use custom function instead of backoff.expo
    Exception,
    max_tries=5,
    on_backoff=log_backoff,
)
def create_list_item(client, user_did, list_uri):
    return client.app.bsky.graph.listitem.create(
        repo=client.me.did,
        record={
            'subject': user_did,
            'list': list_uri,
            'createdAt': client.get_current_time_iso(),
        }
    )


def main():
    print("Bluesky Blocklist Manager")
    print("=" * 25)
    
    # Get credentials
    handle = input("Enter your Bluesky handle (e.g., username.bsky.social): ").strip()
    app_password = getpass.getpass("Enter your app password: ")
    
    # Get list URI/DID
    list_uri = input("Enter the list URI or AT-URI (starts with at://): ").strip()
    
    # Validate list URI format
    if not list_uri.startswith('at://'):
        print("Error: List identifier should be an AT-URI starting with 'at://'")
        sys.exit(1)
    
    # Get DIDs to add
    user_dids = get_user_dids()
    
    if not user_dids:
        print("No valid DIDs provided. Exiting.")
        sys.exit(1)
    
    print(f"\nFound {len(user_dids)} DIDs to add to the list.")
    
    print("\nConnecting to Bluesky...")
    client = Client()
    client.login(handle, app_password)
    print("✓ Successfully authenticated")
    
    already_listed = fetch_blocklist_dids(client, list_uri)
    
    print(f"List already contains {len(already_listed)} DIDs")
    
    user_dids = list(set(user_dids) - set(already_listed))
    
    print(f"DIDs to be added to list: {len(user_dids)}")
    
    # Confirm before proceeding
    confirm = input(f"Add {len(user_dids)} users to list {list_uri}? (y/N): ").strip().lower()
    if confirm != 'y':
        print("Operation cancelled.")
        sys.exit(0)
    
    try:
        # Connect to Bluesky

        
        # Add users to the list
        successful = 0
        failed = 0
        
        for i, user_did in enumerate(user_dids, 1):
            try:
                print(f"Adding user {i}/{len(user_dids)}: {user_did}")
                create_list_item(client, user_did, list_uri)
                successful += 1
                print(f"✓ Added successfully")
                
            except Exception as e:
                print(f"✗ Failed to add {user_did}: {e}")
                failed += 1
        
        # Summary
        print(f"\nOperation complete!")
        print(f"Successfully added: {successful}")
        print(f"Failed: {failed}")
        
        if successful > 0:
            print(f"\n✓ {successful} users have been added to your blocklist.")
        
    except Exception as e:
        print(f"\nError: {e}")
        sys.exit(1)


if __name__ == "__main__":
    main()