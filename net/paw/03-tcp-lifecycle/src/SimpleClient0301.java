import java.io.InputStream;
import java.net.InetSocketAddress;
import java.net.Socket;
import java.nio.charset.StandardCharsets;
import java.time.LocalTime;

/**
 * 원서 3장 Client0301·Client0302 의 대역. 원서에 소스 전문이 없어 동작만 맞춰 새로 썼다.
 *
 * 사용: java SimpleClient0301.java [--host server] [--port 8082]
 *        [--close-after 0|초|never] [--read-delay 초] [--rcvbuf 바이트] [--abort]
 *
 *   --close-after never  원서 Client0302 처럼 끝내 close() 를 부르지 않는다
 *   --read-delay         읽기 시작 전에 멈춘다(받는 쪽이 못 읽어 가는 상황)
 *   --rcvbuf             connect 전에 수신 버퍼를 줄인다
 *   --abort              SO_LINGER 0 으로 연결 직후 중단한다
 */
public class SimpleClient0301 {

    public static void main(String[] args) throws Exception {
        String host = arg(args, "--host", "server");
        int port = Integer.parseInt(arg(args, "--port", "8082"));
        String closeAfter = arg(args, "--close-after", "0");
        int readDelay = Integer.parseInt(arg(args, "--read-delay", "0"));
        int rcvbuf = Integer.parseInt(arg(args, "--rcvbuf", "0"));
        boolean abort = has(args, "--abort");

        Socket socket = new Socket();
        // 수신 버퍼 크기는 handshake 에서 윈도우 스케일이 정해지기 전에 걸어야 반영된다
        if (rcvbuf > 0) {
            socket.setReceiveBufferSize(rcvbuf);
        }

        // 연결
        log("connect " + host + ":" + port);
        socket.connect(new InetSocketAddress(host, port));
        log("connected " + socket.getLocalSocketAddress() + " -> " + socket.getRemoteSocketAddress());

        // 폐기
        if (abort) {
            socket.setSoLinger(true, 0);
            log("SO_LINGER 0 으로 close() 호출");
            socket.close();
            return;
        }

        // 준비시간이 필요한 경우 대기
        if (readDelay > 0) {
            log("읽기 전 " + readDelay + "초 대기");
            Thread.sleep(readDelay * 1000L);
        }

        // 받은 데이터 읽기 — read() 의 -1 이 상대의 FIN 도착이다
        InputStream in = socket.getInputStream();
        byte[] buf = new byte[8192];
        long total = 0;
        StringBuilder head = new StringBuilder();
        int n;
        while ((n = in.read(buf)) != -1) {
            if (head.length() < 64) {
                head.append(new String(buf, 0, Math.min(n, 64), StandardCharsets.US_ASCII));
            }
            total += n;
        }
        log("read() 가 -1 반환 — 받은 바이트 " + total + (total <= 64 ? ", 내용 " + head.toString().strip() : ""));

        // never로 하면 무한진행
        if (closeAfter.equals("never")) {
            log("close() 를 부르지 않고 멈춰 있습니다 (Ctrl+C 로 종료)");
            Thread.sleep(Long.MAX_VALUE);
        }
        
        // 시간초뒤 종료
        int seconds = Integer.parseInt(closeAfter);
        if (seconds > 0) {
            log(seconds + "초 뒤 close() 호출 예정");
            Thread.sleep(seconds * 1000L);
        }
        socket.close();
        log("close() 호출");
    }

    private static String arg(String[] args, String name, String fallback) {
        for (int i = 0; i < args.length - 1; i++) {
            if (args[i].equals(name)) {
                return args[i + 1];
            }
        }
        return fallback;
    }

    private static boolean has(String[] args, String name) {
        for (String a : args) {
            if (a.equals(name)) {
                return true;
            }
        }
        return false;
    }

    private static void log(String message) {
        System.out.println("[client " + LocalTime.now() + "] " + message);
    }
}
