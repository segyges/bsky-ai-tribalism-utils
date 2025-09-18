# AI Tribalism related utils

Goals:
1) AI Blocklist subscriber blocklist
2) AI voluntarily labeller
3) AI general-purpose mod system (big stretch and likely not in this repo)

Current goal is 1), overlaps heavily in approach with 2

## Inverse blocklist success criteria

Input is a list of bluesky blocklist URLs
Result is that all subscribers to those blocklists get pushed to a blocklist on Bluesky

For a stretch criteria, keep the option to tail the firehose rather than hammering constellation