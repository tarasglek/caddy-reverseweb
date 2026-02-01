#!/usr/bin/env python3
import json
import sys
import socket
from pathlib import Path
from typing import Any

"""
- `--ro <path>`: Allow read-only access to specified path (can be specified multiple times or as comma-separated values)
- `--rox <path>`: Allow read-only access with execution to specified path (can be specified multiple times or as comma-separated values)
- `--rw <path>`: Allow read-write access to specified path (can be specified multiple times or as comma-separated values)
- `--rwx <path>`: Allow read-write access with execution to specified path (can be specified multiple times or as comma-separated values)
- `--bind-tcp <port>`: Allow binding to specified TCP port (can be specified multiple times or as comma-separated values)
- `--connect-tcp <port>`: Allow connecting to specified TCP port (can be specified multiple times or as comma-separated values)
- `--env <var>`: Environment variable to pass to the sandboxed command (format: KEY=VALUE or just KEY to pass current value)
- `--best-effort`: Use best effort mode, falling back to less restrictive sandbox if necessary [default: disabled]
- `--log-level <level>`: Set logging level (error, info, debug) [default: "error"]
- `--unrestricted-network`: Allows unrestricted network access (disables all network restrictions)
- `--unrestricted-filesystem`: Allows unrestricted filesystem access (disables all filesystem restrictions)
- `--add-exec`: Automatically adds the executing binary to --rox
- `--ldd`: Automatically adds required libraries to --rox
"""
def wrap_landrun(
    cmd: list[str],
    rwx: list[str] | None = None,
    rw: list[str] | None = None,
    ro: list[str] | None = None,
    rox: list[str] | None = None,
    bind_tcp: list[int] | None = None,
    connect_tcp: list[int] | None = None,
    unrestricted_network: bool = False,
) -> list[str]:
    """Wraps a command with landrun for sandboxing."""
    wrapper = ["landrun"]

    if unrestricted_network:
        wrapper.append("--unrestricted-network")
    if rwx:
        wrapper.extend(["--rwx", ",".join(rwx)])
    if rw:
        wrapper.extend(["--rw", ",".join(rw)])
    if ro:
        wrapper.extend(["--ro", ",".join(ro)])
    if rox:
        wrapper.extend(["--rox", ",".join(rox)])
    if bind_tcp:
        wrapper.extend(["--bind-tcp", ",".join(map(str, bind_tcp))])
    if connect_tcp:
        wrapper.extend(["--connect-tcp", ",".join(map(str, connect_tcp))])

    return wrapper + cmd

def find_free_port() -> int:
    """Finds a free TCP port by binding to port 0."""
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.bind(("", 0))
        return s.getsockname()[1]

def detect_dir(working_dir: Path, port: int) -> list[str] | None:
    """Detects the application type and returns the command to run it."""
    if (working_dir / "main.py").exists():
        return ["uv", "run", "main.py", "--port", str(port)]
    if (working_dir / "main.ts").exists():
        return ["deno", "serve", "--port", str(port), "main.ts"]
    return None

def main() -> None:
    working_dir = Path(sys.argv[1]) if len(sys.argv) > 1 else Path(".")
    if not working_dir.is_dir():
        print(f"Error: directory {working_dir} does not exist", file=sys.stderr)
        sys.exit(1)

    port = find_free_port()
    executable = detect_dir(working_dir, port)

    if not executable:
        executable = ["python3", "-m", "http.server", str(port)]

    # Wrap the executable with landrun for sandboxing
    executable = wrap_landrun(
        executable,
        rwx=[str(working_dir.resolve())],
        bind_tcp=[port]
    )

    result: dict[str, Any] = {
        "executable": executable,
        "reverse_proxy_to": f":{port}",
        "working_directory": str(working_dir.resolve()),
    }

    print(json.dumps(result))

if __name__ == "__main__":
    main()
