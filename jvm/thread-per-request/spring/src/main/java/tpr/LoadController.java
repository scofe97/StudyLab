package tpr;

import java.io.IOException;
import java.lang.management.ManagementFactory;
import java.util.Arrays;
import org.springframework.http.MediaType;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;
import org.springframework.web.servlet.mvc.method.annotation.StreamingResponseBody;

@RestController
public class LoadController {

    private static final byte[] CHUNK = new byte[8192];

    static {
        Arrays.fill(CHUNK, (byte) 'x');
    }

    @GetMapping(value = "/small", produces = MediaType.APPLICATION_JSON_VALUE)
    public String small() {
        return "{\"ok\":true}";
    }

    @GetMapping(value = "/slow", produces = MediaType.APPLICATION_JSON_VALUE)
    public String slow(@RequestParam(defaultValue = "200") long ms) throws InterruptedException {
        // 느린 하위 호출을 흉내 내는 블로킹 대기. 요청을 맡은 스레드가 ms 동안 놀면서 점유된다.
        Thread.sleep(ms);
        return "{\"slept\":" + ms + "}";
    }

    @GetMapping(value = "/big", produces = MediaType.APPLICATION_OCTET_STREAM_VALUE)
    public ResponseEntity<StreamingResponseBody> big(@RequestParam(defaultValue = "1024") int kb) {
        long total = kb * 1024L;
        StreamingResponseBody body = out -> {
            long left = total;
            while (left > 0) {
                int n = (int) Math.min(left, CHUNK.length);
                out.write(CHUNK, 0, n);
                left -= n;
            }
        };
        return ResponseEntity.ok().contentLength(total).body(body);
    }

    /** 플랫폼 스레드 수. 가상 스레드는 ThreadMXBean 에 잡히지 않아 "OS 스레드 수" 비교가 된다. */
    @GetMapping(value = "/stats", produces = MediaType.APPLICATION_JSON_VALUE)
    public String stats() throws IOException {
        int threads = ManagementFactory.getThreadMXBean().getThreadCount();
        return "{\"threads\":" + threads + "}";
    }
}
