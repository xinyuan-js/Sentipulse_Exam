#!/usr/bin/env python3
import json
import sys


def main():
    req = json.load(sys.stdin)
    text = str(req.get("data", {}).get("text", ""))
    json.dump({"result": {"upper": text.upper(), "length": len(text)}}, sys.stdout)


if __name__ == "__main__":
    main()
