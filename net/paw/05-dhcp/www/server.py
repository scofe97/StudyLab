"""5장 묶음 C 의 작은 HTTP 서버.

/fast  바로 응답          /slow  2초 뒤 응답 (서버 처리 시간을 흉내)
/big   2MB 응답 (여러 TCP 세그먼트로 쪼개진다)
POST /login  폼을 받아 200
"""
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


class Handler(BaseHTTPRequestHandler):
    # HTTP/1.1 이어야 연결 하나에 요청 여럿(keep-alive)이 실린다
    protocol_version = "HTTP/1.1"

    def _send(self, body: bytes, code: int = 200):
        self.send_response(code)
        self.send_header("Content-Type", "text/plain")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        if self.path == "/fast":
            self._send(b"fast\n")
        elif self.path == "/slow":
            time.sleep(2)
            self._send(b"slow\n")
        elif self.path == "/big":
            self._send(b"x" * (2 * 1024 * 1024))
        else:
            self._send(b"not found\n", 404)

    def do_POST(self):
        length = int(self.headers.get("Content-Length", 0))
        self.rfile.read(length)
        self._send(b"ok\n")

    def log_message(self, *args):
        pass


ThreadingHTTPServer(("0.0.0.0", 80), Handler).serve_forever()
