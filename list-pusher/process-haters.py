import json
import pprint

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
    haters = file.readlines()

haters = [json.dumps(line) for line in haters]

prime_hater = haters[0]

pprint.pp(json.dumps(prime_hater, indent=4))