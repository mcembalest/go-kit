import json, sys

for line in sys.stdin:
    req = json.loads(line)
    print(json.dumps({"error": req["fail"]} if "fail" in req else {"echo": req}), flush=True)
