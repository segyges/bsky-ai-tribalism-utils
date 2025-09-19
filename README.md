# AI Tribalism related utils

Goals:
1) AI Blocklist subscriber blocklist
2) AI voluntarily labeller
3) AI general-purpose mod system (big stretch and likely not in this repo)

Current goal is 1), overlaps heavily in approach with 2

## Inverse blocklist

### Success criteria

Input is a list of bluesky blocklist URLs
Result is that all subscribers to those blocklists get pushed to a blocklist on Bluesky

For a stretch criteria, keep the option to tail the firehose rather than hammering constellation

### Obstacles

On the happy path, our main obstacle is to try to get Constellation to do a full backfill.

On the unhappy path we're forking Constellation to do a full backfill (and maybe be more efficient by only backfilling records we care about).

### Odd notes

#### Bluessky vs at urls differ

Formatting via bluesky and via constellation is somewhat different, a bluesky list addressed at

https://bsky.app/profile/did:plc:2bij7yypmcuvwyz4gyqwtluy/lists/3lbxfscjqno2d
is queried properly at

at://did:plc:2bij7yypmcuvwyz4gyqwtluy/app.bsky.graph.list/3lbxfscjqno2d

#### Cursor

Cursor behavior is undocumented for the main constellation links api, but needs to be used to query any entire list of records.
