const http = require('http');
const os = require('os');
const fs = require('fs');

const listenPort = 8080;

// 서버가 시작될 때 한 번 호스트명을 읽어 둔다 (컨테이너 ID = 호스트명)
const serverHostname = os.hostname();

function handler(request, response) {
  let clientIP = request.connection.remoteAddress;
  console.log("Received request for " + request.url + " from " + clientIP);

  // HTML 모드 — /html 을 명시적으로 요청할 때만 정적 페이지를 서빙
  if (request.url === '/html') {
    response.writeHead(200, {'Content-Type': 'text/html'});
    let page = fs.readFileSync('html/index.html', 'utf8');
    page = page.replace("_HOSTNAME_", serverHostname);
    page = page.replace("_CLIENT_IP_", clientIP);
    response.end(page);
  } else {
    // 평문(plain-text) 모드 — 기본 응답. curl 등 터미널 클라이언트용
    response.writeHead(200, {'Content-Type': 'text/plain'});
    response.end(
      `Kiada version 0.1. Request processed by "${serverHostname}". Client IP: ${clientIP}\n`
    );
  }
}

const server = http.createServer(handler);
server.listen(listenPort, function () {
  console.log("Kiada 0.1 server starting... listening on port " + listenPort);
});
