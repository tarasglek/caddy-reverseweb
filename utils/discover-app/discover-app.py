#!/usr/bin/env python3
import json
import sys
from pathlib import Path
from typing import Any

# https://raw.githubusercontent.com/Zouuup/landrun/refs/heads/main/README.md landrun flags documented here
def wrap_landrun(cmd:string[], permissions(rwxstring[],rw-ro:string[],bind-tcp:number,)) # complete a function that wraps landruncli and returns it AI!
def main() -> None:
    working_dir = Path(sys.argv[1]) if len(sys.argv) > 1 else Path(".")
    if not working_dir.is_dir():
        print(f"Error: directory {working_dir} does not exist", file=sys.stderr)
        sys.exit(1)

    result: dict[str, Any] = {
        "executable": ["python3", "-m", "http.server", "23232"],
        "reverse_proxy_to": ":23232",
        "working_directory": str(working_dir.resolve()),
    }

    print(json.dumps(result))

if __name__ == "__main__":
    main()
