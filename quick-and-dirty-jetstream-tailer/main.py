import asyncio
import websockets
import json

uri = "wss://jetstream2.us-east.bsky.network/subscribe?wantedCollections=app.bsky.graph.listblock"

async def listen_to_websocket(bl):
  async with websockets.connect(uri) as websocket:
    while True:
      try:
        message = await websocket.recv()
        message = json.loads(message)
        if message['kind'] != 'commit' or not 'commit' in message:
            print(f'Kind is not commit or message does not have a "commit" structure, skipping')
            continue
        if message['commit']['operation'] != 'create':
            print(f'Message type is {message['commit']['operation']}, ie, not create, should really be a delete skipping\nFull message is {message}')
            
        if 'collection' not in message['commit']:
            print(f'This should be unreachable (first such spot)\nFull message is {message}')
            continue
        if message['commit']['collection'] != 'app.bsky.graph.listblock':
            print(f'This should be unreachable (second such spot)\nFull message is {message}')
            continue
        # One hopes this is the happy path
        list_blocked = message['commit']['record']['subject']
        if list_blocked not in bl:
            print("Not in blocklists: {list_blocked}")
            continue
        print(f'Correct type of blocklist subscription! List: {list_blocked}')
        blocker = message['did']
        time = message['commit']['record']['createdAt']
        
        
      except websockets.ConnectionClosed as e:
        print(f"Connection closed: {e}")
        break
      except Exception as e:
        print(f"Error: {e}")


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


if __name__ == '__main__':
  # When run directly, print a short summary of the local blocklist and then
  # start the websocket listener.
  bl = load_blocklist()
  print(f"Loaded {len(bl)} blocklist entries; sample:", bl[:3])
  asyncio.get_event_loop().run_until_complete(listen_to_websocket(bl))