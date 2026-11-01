"""Framed stdio transport to the loopback API inside authenticated G112."""

import argparse
import base64
import json
import re
import sys
import zlib
from pathlib import Path
from urllib.error import HTTPError
from urllib.request import ProxyHandler, Request, build_opener


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", required=True)
    parser.add_argument("--port", type=int, default=8991)
    args = parser.parse_args()
    token = (Path(args.root) / "http-key").read_text().strip()
    transport = build_opener(ProxyHandler({}))
    incoming = {}
    outgoing = None
    print("TEAM_RELAY_READY", flush=True)
    for line in sys.stdin:
        try:
            item = json.loads(line)
            nonce = item["id"]
            if not re.fullmatch(r"[a-f0-9]{32}", nonce):
                raise ValueError("invalid request identity")
            if "part" in item:
                if not outgoing or outgoing[0] != nonce:
                    raise ValueError("response identity no longer current")
                part = item["part"]
            else:
                if item["index"] == 0:
                    incoming = {"id": nonce, "parts": item["parts"], "data": []}
                if (
                    incoming.get("id") != nonce
                    or item["index"] != len(incoming["data"])
                    or not 1 <= incoming["parts"] <= 100
                ):
                    raise ValueError("invalid input sequence")
                incoming["data"].append(item["data"])
                print(f"TEAM_IN {nonce} {item['index']} END", flush=True)
                if len(incoming["data"]) != incoming["parts"]:
                    continue
                request = json.loads(
                    zlib.decompress(base64.b64decode("".join(incoming["data"])))
                )
                if request["method"] not in ["GET", "POST"] or not request[
                    "path"
                ].startswith("/api/"):
                    raise ValueError("unsupported request")
                body = (
                    json.dumps(request["body"], ensure_ascii=False).encode()
                    if request["method"] == "POST"
                    else None
                )
                req = Request(
                    "http://127.0.0.1:" + str(args.port) + request["path"],
                    data=body,
                    method=request["method"],
                    headers={
                        "Authorization": "Bearer " + token,
                        "Content-Type": "application/json",
                    },
                )
                try:
                    with transport.open(req, timeout=25) as response:
                        value = {"status": response.status, "body": json.load(response)}
                except HTTPError as error:
                    value = {"status": error.code, "body": json.load(error)}
                payload = zlib.compress(json.dumps(value, ensure_ascii=False).encode())
                outgoing = (
                    nonce,
                    [payload[i : i + 1800] for i in range(0, len(payload), 1800)],
                )
                incoming = {}
                part = 0
            chunk = outgoing[1][part]
            print(
                f"TEAM_DATA {nonce} {part} {len(outgoing[1])} {base64.b64encode(chunk).decode()} END",
                flush=True,
            )
        except Exception:
            # Unknown POST results are preserved, never automatically replayed.
            print("TEAM_RELAY_ERROR", file=sys.stderr, flush=True)


if __name__ == "__main__":
    main()
