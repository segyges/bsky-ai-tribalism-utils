import asyncio
import websockets
import json
import os
import csv

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
            print(f"Not in blocklists: {list_blocked}")
            continue
        print(f'Correct type of blocklist subscription! List: {list_blocked}')
        blocker = message.get('did')
        time = message['commit']['record'].get('createdAt')
        # append to CSV (blocker,list_blocked,time)
        try:
          await append_to_csv(blocker, list_blocked, time)
        except Exception as e:
          print(f"Failed to write CSV row: {e}")
        
        
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


def _get_csv_path(path=None):
  if path:
    return path
  here = os.path.dirname(__file__)
  return os.path.join(here, 'blocks.csv')


def _write_csv_row(path, row):
  # ensure directory exists (should be current folder)
  dirn = os.path.dirname(path)
  if dirn and not os.path.exists(dirn):
    os.makedirs(dirn, exist_ok=True)
  write_header = not os.path.exists(path) or os.path.getsize(path) == 0
  with open(path, 'a', newline='', encoding='utf-8') as f:
    w = csv.writer(f)
    if write_header:
      w.writerow(['blocker', 'list_blocked', 'time'])
    w.writerow(row)


async def append_to_csv(blocker, list_blocked, time, path=None):
  """Append a row to CSV in a thread to avoid blocking the event loop."""
  csv_path = _get_csv_path(path)
  row = [blocker or '', list_blocked or '', time or '']
  # run blocking IO in a separate thread
  try:
    await asyncio.to_thread(_write_csv_row, csv_path, row)
  except AttributeError:
    # asyncio.to_thread only exists in Python 3.9+; fallback to run_in_executor
    loop = asyncio.get_running_loop()
    await loop.run_in_executor(None, _write_csv_row, csv_path, row)


if __name__ == '__main__':
  # When run directly, print a short summary of the local blocklist and then
  # start the websocket listener.
  bl = load_blocklist()
  print(f"Loaded {len(bl)} blocklist entries; sample:", bl[:3])
  asyncio.get_event_loop().run_until_complete(listen_to_websocket(bl))