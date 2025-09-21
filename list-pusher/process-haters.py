import json
from collections import defaultdict

## This is only going to be used one single time to process a dump from clearsky, if we are at all lucky, and is preserved in this repo for the sole purpose of reproducibility in case something goes horribly wrong later.


# structure is:
    
#     {
#   "identity": "str",
#   "data": [
#     "dict",
#     {
#       "description": "str",
#       "list_name": "str",
#       "list_owner": "str",
#       "list_url": "str",
#       "users": [
#         "list",
#         [
#           "dict",
#           {
#             "date_added": "str",
#             "did": "str"
#           }
#         ]
#       ]
#     }
#   ]
# } 

with open('haters.jsonl', 'r') as file:
    haters_lines = file.readlines()

def unwrap_dict(maybe):
    # handle ["dict", {...}] or raw dict
    if isinstance(maybe, list) and len(maybe) >= 2 and maybe[0] == "dict" and isinstance(maybe[1], dict):
        return maybe[1]
    if isinstance(maybe, dict):
        return maybe
    return None

def unwrap_list(maybe):
    # handle ["list", [...]] or raw list
    if isinstance(maybe, list) and len(maybe) >= 2 and maybe[0] == "list" and isinstance(maybe[1], list):
        return maybe[1]
    if isinstance(maybe, list):
        return maybe
    return []

# map did -> list of [list_url, date_added]
user_map = {}

for line in haters_lines:
    line = line.strip()
    if not line:
        continue
    try:
        record = json.loads(line)
    except Exception:
        continue

    data_field = record.get('data')
    data_obj = unwrap_dict(data_field)
    if not data_obj:
        continue

    list_url = data_obj.get('list_url')
    if not list_url:
        continue

    users_field = data_obj.get('users', [])
    users_list = unwrap_list(users_field)

    for entry in users_list:
        user_obj = unwrap_dict(entry)
        if not user_obj:
            continue
        did = user_obj.get('did')
        date_added = user_obj.get('date_added')
        # skip entries without a DID
        if not did:
            continue
        # store or overwrite (keeps last-seen list/date for that DID)
        # append this (list_url, date_added) pair to the DID's list
        user_map.setdefault(did, []).append([list_url, date_added])

# write resulting mapping to file and print total users
out = user_map
with open('processed_haters.json', 'w') as outf:
    json.dump(out, outf, indent=2)

total_users = len(out)
print(total_users)