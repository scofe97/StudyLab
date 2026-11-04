package tpr;

import jakarta.servlet.http.HttpServletResponse;
import java.io.IOException;
import java.io.OutputStream;
import java.nio.charset.StandardCharsets;
import java.time.Instant;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

/**
 * 요청 스레드가 연결이 끊길 때까지 루프를 돌며 쓰고 잔다. 가장 단순한 요청당 스레드 모양이다.
 * 플랫폼 스레드 풀이면 연결 수가 풀 크기에서 막히고, 가상 스레드면 같은 코드가 수천 연결을 버틴다.
 */
@RestController
@ConditionalOnProperty(name = "tpr.sse.mode", havingValue = "blocking", matchIfMissing = true)
public class SseBlockingController {

    @GetMapping("/sse")
    public void sse(@RequestParam(defaultValue = "1000") long interval, HttpServletResponse res)
            throws IOException, InterruptedException {
        res.setContentType("text/event-stream");
        res.setCharacterEncoding("UTF-8");
        res.setHeader("Cache-Control", "no-cache");
        OutputStream out = res.getOutputStream();
        try {
            while (true) {
                out.write(("data: " + epochNanos() + "\n\n").getBytes(StandardCharsets.UTF_8));
                out.flush(); // 클라이언트가 끊겼다면 여기서 IOException 이 나서 루프가 끝난다.
                Thread.sleep(interval);
            }
        } catch (IOException disconnected) {
            // 연결 종료 — 정상 경로
        }
    }

    static long epochNanos() {
        Instant now = Instant.now();
        return now.getEpochSecond() * 1_000_000_000L + now.getNano();
    }
}
