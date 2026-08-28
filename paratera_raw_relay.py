#!/usr/bin/env python3
import json
import os
import ssl
import hashlib
import http.client
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import urlsplit


UPSTREAM_HOST = os.environ.get("UPSTREAM_HOST", "llmapi.paratera.com")
UPSTREAM_PORT = int(os.environ.get("UPSTREAM_PORT", "443"))
UPSTREAM_API_KEY = os.environ["UPSTREAM_API_KEY"]
CLIENT_API_KEY = os.environ.get("CLIENT_API_KEY", UPSTREAM_API_KEY)
MAX_BODY_BYTES = int(os.environ.get("MAX_BODY_BYTES", str(32 * 1024 * 1024)))
MODEL_ALIASES = {
    "gpt-5.6-luna": "GPT-5.6-Luna",
    "gpt-5.6-sol": "GPT-5.6-Sol",
    "gpt-5.6-terra": "GPT-5.6-Terra",
}


def json_response(status, payload):
    return status, json.dumps(payload, ensure_ascii=False).encode()


class RelayHandler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def log_message(self, format_string, *args):
        print("%s - %s" % (self.address_string(), format_string % args), flush=True)

    def do_GET(self):
        if self.path == "/healthz":
            self._send_bytes(*json_response(200, {"status": "ok"}), content_type="application/json")
            return
        self._forward()

    def do_HEAD(self):
        self._forward(head_only=True)

    def do_POST(self):
        self._forward()

    def _send_bytes(self, status, body, content_type="application/json"):
        self.send_response(status)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(body)))
        self.send_header("Cache-Control", "no-store")
        self.end_headers()
        if self.command != "HEAD":
            self.wfile.write(body)

    def _unauthorized(self):
        self._send_bytes(401, b'{"error":{"message":"Invalid API key","type":"authentication_error"}}')

    def _forward(self, head_only=False):
        parsed = urlsplit(self.path)
        if not (parsed.path == "/v1" or parsed.path.startswith("/v1/")):
            self._send_bytes(404, b'{"error":{"message":"Only /v1 endpoints are available"}}')
            return
        authorization = self.headers.get("Authorization", "")
        if authorization != "Bearer " + CLIENT_API_KEY:
            self._unauthorized()
            return

        body = b""
        content_length = int(self.headers.get("Content-Length", "0"))
        if content_length < 0 or content_length > MAX_BODY_BYTES:
            self._send_bytes(413, b'{"error":{"message":"Request body too large"}}')
            return
        if content_length:
            body = self.rfile.read(content_length)
            if len(body) != content_length:
                self._send_bytes(400, b'{"error":{"message":"Incomplete request body"}}')
                return
            body = self._map_model_alias(body)

        request_headers = {}
        hop_by_hop = {
            "authorization",
            "connection",
            "content-length",
            "host",
            "keep-alive",
            "proxy-authenticate",
            "proxy-authorization",
            "te",
            "trailer",
            "transfer-encoding",
            "upgrade",
            "accept-encoding",
        }
        for key, value in self.headers.items():
            if key.lower() not in hop_by_hop:
                request_headers[key] = value
        request_headers["Authorization"] = "Bearer " + UPSTREAM_API_KEY
        request_headers["Accept-Encoding"] = "identity"

        connection = None
        try:
            connection = http.client.HTTPSConnection(
                UPSTREAM_HOST,
                UPSTREAM_PORT,
                timeout=300,
                context=ssl.create_default_context(),
            )
            connection.request(self.command, parsed.path + ("?" + parsed.query if parsed.query else ""), body=body or None, headers=request_headers)
            response = connection.getresponse()
            response_body_length = response.getheader("Content-Length")
            self.send_response(response.status, response.reason)
            response_hop_by_hop = {"connection", "content-length", "keep-alive", "proxy-authenticate", "proxy-authorization", "te", "trailer", "transfer-encoding", "upgrade"}
            for key, value in response.getheaders():
                if key.lower() not in response_hop_by_hop:
                    self.send_header(key, value)
            if response_body_length is not None:
                self.send_header("Content-Length", response_body_length)
            else:
                self.send_header("Transfer-Encoding", "chunked")
            self.send_header("X-Raw-Relay", "paratera")
            self.end_headers()
            if not head_only and self.command != "HEAD":
                if response_body_length is None:
                    while chunk := response.read(64 * 1024):
                        self.wfile.write(('%X\r\n' % len(chunk)).encode())
                        self.wfile.write(chunk)
                        self.wfile.write(b"\r\n")
                    self.wfile.write(b"0\r\n\r\n")
                else:
                    while chunk := response.read(64 * 1024):
                        self.wfile.write(chunk)
        except Exception as error:
            digest = hashlib.sha256(str(error).encode()).hexdigest()[:12]
            if not self.wfile.closed:
                try:
                    self._send_bytes(502, json.dumps({"error": {"message": "Upstream request failed", "type": "upstream_error", "request_id": digest}}).encode())
                except (BrokenPipeError, ConnectionResetError):
                    pass
        finally:
            if connection is not None:
                connection.close()

    @staticmethod
    def _map_model_alias(body):
        try:
            payload = json.loads(body)
        except (TypeError, ValueError):
            return body
        model = payload.get("model")
        if isinstance(model, str):
            payload["model"] = MODEL_ALIASES.get(model.lower(), model)
        return json.dumps(payload, ensure_ascii=False, separators=(",", ":")).encode()


class RelayServer(ThreadingHTTPServer):
    daemon_threads = True
    allow_reuse_address = True


def main():
    host = os.environ.get("LISTEN_HOST", "0.0.0.0")
    port = int(os.environ.get("LISTEN_PORT", "8320"))
    server = RelayServer((host, port), RelayHandler)
    print("paratera raw relay listening on %s:%d -> https://%s:%d" % (host, port, UPSTREAM_HOST, UPSTREAM_PORT), flush=True)
    server.serve_forever()


if __name__ == "__main__":
    main()
