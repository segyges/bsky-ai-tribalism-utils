import asyncio
import websockets
import json
import os
import csv
import time

BASE_URI = "wss://jetstream2.us-west.bsky.network/subscribe"
recent_timestamp = None

async def listen_to_websocket(bl, uri):
    global recent_timestamp
    async with websockets.connect(uri) as websocket:
        while True:
            try:
                message = await websocket.recv()
                message = json.loads(message)
                
                # Fix the timestamp logic
                if 'time_us' in message:
                    current_time = message['time_us']
                    if recent_timestamp is None or current_time > recent_timestamp:
                        recent_timestamp = current_time
           
                if message['kind'] != 'commit' or 'commit' not in message:
                    print(f'Kind is not commit or message does not have a "commit" structure, skipping')
                    continue
                    
                if message['commit']['operation'] != 'create':
                    print(f'Message type is {message["commit"]["operation"]}, ie, not create, should really be a delete, logging to our outfile')
                    write_to_unblocks(message)
                    continue  # Add continue here so we don't process deletes as creates
                    
                if 'collection' not in message['commit']:
                    print(f'This should be unreachable (first such spot)\nFull message is {message}')
                    continue
                    
                if message['commit']['collection'] != 'app.bsky.graph.listblock':
                    print(f'This should be unreachable (second such spot)\nFull message is {message}')
                    continue
                    
                # One hopes this is the happy path
                list_blocked = message['commit']['record']['subject']
                if list_blocked not in bl:
                    print(f"Not in blocklists: {list_blocked}")
                    continue
                    
                print(f'Correct type of blocklist subscription! List: {list_blocked}')
                write_to_known_blocks(message)
           
            except websockets.ConnectionClosed as e:
                print(f"Connection closed: {e}")
                break
            except Exception as e:
                print(f"Error: {e}")

def write_to_known_blocks(message):
    """Write relevant block messages to relevant_blocks.jsonl"""
    with open('relevant_blocks.jsonl', 'a') as f:
        f.write(json.dumps(message) + '\n')

def write_to_unblocks(message):
    """Write block deletion messages to all_block_deletions.jsonl"""
    with open('all_block_deletions.jsonl', 'a') as f:
        f.write(json.dumps(message) + '\n')

def load_blocklist(path=None):
    """Load anti-ai-lists.txt into a list of strings.
    - path: optional path to the file; if None, uses the bundled file in this folder.
    - ignores blank lines and lines starting with '#'
    - strips whitespace from each line
    """
    import os
    if path is None:
        here = os.path.dirname(__file__)
        path = os.path.join(here, 'anti-ai-lists.txt')
    items = []
    try:
        with open(path, 'r', encoding='utf-8') as f:
            for raw in f:
                line = raw.strip()
                if not line:
                    continue
                if line.startswith('#'):
                    continue
                items.append(line)
    except FileNotFoundError:
        # Return empty list if file isn't present; caller can decide how to handle
        return []
    return items

async def periodic_disk_writer():
    while True:
        await asyncio.sleep(60)  # 60 seconds
        if recent_timestamp is not None:
            with open('last_run_timestamp.txt', 'w') as f:
                f.write(str(recent_timestamp))
            print(f"Wrote timestamp to disk: {recent_timestamp}")

async def main():
    # When run directly, print a short summary of the local blocklist and then
    # start the websocket listener.
    bl = load_blocklist()
    print(f"Loaded {len(bl)} blocklist entries; sample:", bl[:3])
    
    if os.path.exists("last_run_timestamp.txt"):
        with open('last_run_timestamp.txt', 'r') as file:
            # Read in timestamp, set back a full hour to ensure we don't miss anything
            cursor = int(file.read().strip()) - 1000*1000*60*60
    else:
        cursor = int(time.time() * 1000000) - (60 * 60 * 1000 * 1000)
    
    print(f"Starting from cursor: {cursor}")
    
    # Start the periodic writer as a background task
    asyncio.create_task(periodic_disk_writer())
    
    # Run the main websocket listener
    await listen_to_websocket(bl, BASE_URI + "?wantedCollections=app.bsky.graph.listblock" + f'&cursor={cursor}')

if __name__ == '__main__':
    asyncio.run(main())