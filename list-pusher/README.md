# The Rundown

This is quick and dirty, and it does the following:

1) scrape_clearsky_blocklist_api.py scrapes clearsky for all members of a blocklist, outputting haters.jsonl
2) process-haters.py processes the haters.jsonl to output processed_haters.json, which is a clean list of DIDs with what blocklist subs they have
3) main.go pushes processed_haters.json up as a blocklist, with lots of complex backoff