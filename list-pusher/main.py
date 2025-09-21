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
    
    # Confirm before proceeding
    confirm = input(f"Add {len(user_dids)} users to list {list_uri}? (y/N): ").strip().lower()
    if confirm != 'y':
        print("Operation cancelled.")
        sys.exit(0)
    
    try:
        # Connect to Bluesky
        print("\nConnecting to Bluesky...")
        client = Client()
        client.login(handle, app_password)
        print("✓ Successfully authenticated")
        
        # Add users to the list
        successful = 0
        failed = 0
        
        for i, user_did in enumerate(user_dids, 1):
            try:
                print(f"Adding user {i}/{len(user_dids)}: {user_did}")
                
                client.app.bsky.graph.listitem.create(
                    repo=client.me.did,
                    record={
                        'subject': user_did,
                        'list': list_uri,
                        'createdAt': client.get_current_time_iso(),
                    }
                )
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