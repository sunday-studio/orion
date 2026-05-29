import json
import os
import threading
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


PORT = int(os.environ.get("ORION_EXAMPLE_PORT", "8080"))
FAIL_FILE = os.environ.get("ORION_EXAMPLE_FAIL_FILE", "/tmp/orion-example-fail")


def background_worker() -> None:
    while True:
        time.sleep(30)


class Handler(BaseHTTPRequestHandler):
    def do_GET(self) -> None:
        if self.path != "/health":
            self.send_response(404)
            self.end_headers()
            return

        unhealthy = os.path.exists(FAIL_FILE)
        payload = {
            "service": "python-sleep-example",
            "status": "down" if unhealthy else "up",
            "fail_file": FAIL_FILE,
        }
        body = json.dumps(payload).encode("utf-8")

        self.send_response(503 if unhealthy else 200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, format: str, *args: object) -> None:
        return


if __name__ == "__main__":
    worker = threading.Thread(target=background_worker, daemon=True)
    worker.start()

    server = ThreadingHTTPServer(("0.0.0.0", PORT), Handler)
    server.serve_forever()
