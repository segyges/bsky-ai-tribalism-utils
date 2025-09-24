import json
import requests
import time
from urllib.parse import urlparse

def main():
    # Configuration
    input_file = 'links.txt'  # Input file containing links
    output_file = 'haters.jsonl'  # Output JSONL file

    # Read links from input file
    with open(input_file, 'r') as f:
        links = [line.strip() for line in f if line.strip()]

    # Process each link
    for link in links:
        print(f"Processing list: {link}")
        # Extract base URL without page number
        parsed = urlparse(link)
        base_url = parsed.scheme + '://' + parsed.netloc + parsed.path
        base_url = base_url.rsplit('/', 1)[0]  # Remove trailing page number

        page = 1
        while True:
            # Construct page URL
            page_url = f"{base_url}/{page}"
            print(f"  Fetching page {page}...")

            try:
                response = requests.get(page_url, timeout=10)
                response.raise_for_status()
                data = response.json()
            except Exception as e:
                print(f"  Error fetching {page_url}: {e}")
                break

            # Validate JSON structure
            if 'data' not in data or 'users' not in data['data']:
                print(f"  Invalid JSON structure at {page_url}")
                break

            # Write to JSONL file
            with open(output_file, 'a') as outfile:
                json.dump(data, outfile)
                outfile.write('\n')

            # Check for empty users array
            if not data['data']['users']:
                print(f"  Empty users array found at page {page}. Moving to next list.")
                break

            page += 1
            print(f"Grabbed {page_url}")
            time.sleep(2)  # Be polite between requests

    print("All lists processed successfully.")

if __name__ == '__main__':
    main()
