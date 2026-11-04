package tpr;

import java.io.IOException;
import java.util.concurrent.Executors;
import java.util.concurrent.ScheduledExecutorService;
import java.util.concurrent.ScheduledFuture;
import java.util.concurrent.TimeUnit;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;
import org.springframework.web.servlet.mvc.method.annotation.SseEmitter;

/**
 * SseEmitter 로 요청 스레드를 곧바로 놓아준다(servlet async). 연결마다 다음 이벤트를 보내는 일은
 * 공용 스케줄러 스레드 하나가 맡는다. 요청당 스레드의 예외가 되는 이유가 이것이다.
 */
@RestController
@ConditionalOnProperty(name = "tpr.sse.mode", havingValue = "emitter")
public class SseEmitterController {

    private final ScheduledExecutorService scheduler = Executors.newSingleThreadScheduledExecutor(r -> {
        Thread t = new Thread(r, "sse-scheduler");
        t.setDaemon(true);
        return t;
    });

    @GetMapping("/sse")
    public SseEmitter sse(@RequestParam(defaultValue = "1000") long interval) {
        SseEmitter emitter = new SseEmitter(0L); // 0 = 만료 없음
        ScheduledFuture<?> task = scheduler.scheduleAtFixedRate(() -> {
            try {
                emitter.send(SseEmitter.event().data(SseBlockingController.epochNanos()));
            } catch (IOException | IllegalStateException gone) {
                emitter.complete();
            }
        }, 0, interval, TimeUnit.MILLISECONDS);
        Runnable stop = () -> task.cancel(false);
        emitter.onCompletion(stop);
        emitter.onTimeout(stop);
        emitter.onError(e -> stop.run());
        return emitter;
    }
}
