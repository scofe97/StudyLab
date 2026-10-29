import java.io.IOException;
import java.io.OutputStream;
import java.net.ServerSocket;
import java.net.Socket;
import java.nio.charset.StandardCharsets;
import java.time.LocalTime;

/**
 * 원서 3장 TCPServer01·Server0302 의 대역.
 * 원서는 파일 이름과 socket.close() 조각만 싣고 소스 전문이 없어, 원서가 서술한 동작만 맞춰 새로 썼다.
 *
 * 사용: java SimpleTCPServer01.java [--port 8082] [--mode normal|slow|bulk] [--delay-ms 3000] [--bytes 5242880]
 */
public class SimpleTCPServer01 {

    // 줄바꿈까지 29바이트 — 노트 03-01 §4 packet#4 의 29바이트와 대조하려고 맞췄다(원문 hex 는 앞부분만 보여 끝 바이트는 추정)
    private static final byte[] GREETING = "ANISH NATH NORMAL CONNECTION\n".getBytes(StandardCharsets.US_ASCII);

    public static void main(String[] args) throws Exception {
        // 옵션
        int port = Integer.parseInt(arg(args, "--port", "8082"));
        String mode = arg(args, "--mode", "normal");
        long delayMs = Long.parseLong(arg(args, "--delay-ms", "3000"));
        long bulkBytes = Long.parseLong(arg(args, "--bytes", "5242880"));

        // 듣기
        try (ServerSocket listener = new ServerSocket(port)) {
            log("LISTEN :" + port + " (mode=" + mode + ")");
            // 연결마다 처리
            while (true) {
                Socket socket = listener.accept();
                log("accept " + socket.getRemoteSocketAddress());
                // 연결 하나가 비정상으로 끝나도 서버는 살아 있어야 다음 시나리오를 이어 돌릴 수 있다
                try {
                    handle(socket, mode, delayMs, bulkBytes);
                } catch (IOException e) {
                    log("연결 처리 중 예외 — " + e);
                }
            }
        }
    }

    private static void handle(Socket socket, String mode, long delayMs, long bulkBytes) throws IOException, InterruptedException {
        // 모든 모드에서 서버가 먼저 close() 한다 — 원서 normal-connection.pcap·close_wait.pcap 이 서버가 먼저 닫는 모양이다
        try (socket) {
            OutputStream out = socket.getOutputStream();
            switch (mode) {
                // 29바이트 한 번 보내기
                case "normal" -> out.write(GREETING);
                // 서버 처리 지연 — handshake 는 빠르고 데이터만 늦는 경우
                case "slow" -> {
                    log("응답 전 " + delayMs + "ms 처리 지연");
                    Thread.sleep(delayMs);
                    out.write(GREETING);
                }
                // 대량 전송 — 윈도우와 손실을 보는 모드
                case "bulk" -> {
                    byte[] chunk = new byte[64 * 1024];
                    long sent = 0;
                    while (sent < bulkBytes) {
                        int n = (int) Math.min(chunk.length, bulkBytes - sent);
                        out.write(chunk, 0, n);
                        sent += n;
                    }
                    log("보냄 " + sent + " bytes");
                }
                default -> throw new IllegalArgumentException("알 수 없는 mode: " + mode);
            }
            out.flush();
            log("close() 호출");
        }
    }

    private static String arg(String[] args, String name, String fallback) {
        for (int i = 0; i < args.length - 1; i++) {
            if (args[i].equals(name)) {
                return args[i + 1];
            }
        }
        return fallback;
    }

    private static void log(String message) {
        System.out.println("[server " + LocalTime.now() + "] " + message);
    }
}
