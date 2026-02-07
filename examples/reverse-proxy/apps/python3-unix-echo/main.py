#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.13"
# ///
import http.server
import os
import sys
import socket

class UnixHTTPServer(http.server.HTTPServer):
    address_family = socket.AF_UNIX

    def server_bind(self):
        self.socket.bind(self.server_address)

class EchoHandler(http.server.BaseHTTPRequestHandler):
    def address_string(self):
        # For Unix sockets, client_address is often an empty string or a path string.
        # The default implementation tries to access self.client_address[0], which fails.
        if isinstance(self.client_address, (str, bytes)):
            return str(self.client_address) or "unix"
        if not self.client_address or not hasattr(self.client_address, '__getitem__'):
            return "unix"
        try:
            return super().address_string()
        except (IndexError, TypeError):
            return "unix"

    def do_GET(self):
        self.send_response(200)
        self.send_header('Content-type', 'text/plain')
        self.end_headers()
        
        response = f"Request Headers:\n{self.headers}\nLocation: {self.path}"
        self.wfile.write(response.encode('utf-8'))

    def do_HEAD(self):
        self.send_response(200)
        self.end_headers()

if __name__ == "__main__":
    # Use REVERSE_PROXY_TO environment variable
    addr_str = os.environ.get("REVERSE_PROXY_TO")
    if not addr_str or not addr_str.startswith("unix/"):
        print("Error: REVERSE_PROXY_TO must be set to a unix/ path", file=sys.stderr)
        sys.exit(1)

    socket_path = addr_str[5:]
    if os.path.exists(socket_path):
        os.remove(socket_path)
    
    server_address = socket_path
    httpd = UnixHTTPServer(server_address, EchoHandler)
    print(addr_str)

    # Signal readiness to Caddy by printing the address to stdout
    sys.stdout.flush()
    
    try:
        httpd.serve_forever()
    finally:
        if os.path.exists(socket_path):
            os.remove(socket_path)
