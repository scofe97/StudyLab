package tpr;

import static org.assertj.core.api.Assertions.assertThat;

import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.util.stream.Stream;
import org.junit.jupiter.api.Test;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.test.web.server.LocalServerPort;

@SpringBootTest(webEnvironment = SpringBootTest.WebEnvironment.RANDOM_PORT, properties = "tpr.sse.mode=emitter")
class SseEmitterTest {

    @LocalServerPort
    int port;

    @Test
    void sse는_첫_이벤트를_바로_보낸다() throws Exception {
        // Given — emitter 모드
        HttpRequest req = HttpRequest.newBuilder(URI.create("http://localhost:" + port + "/sse?interval=100")).build();

        // When
        HttpResponse<Stream<String>> res = HttpClient.newHttpClient().send(req, HttpResponse.BodyHandlers.ofLines());
        String first = res.body().findFirst().orElseThrow();
        res.body().close();

        // Then
        assertThat(res.statusCode()).isEqualTo(200);
        assertThat(res.headers().firstValue("Content-Type").orElse("")).startsWith("text/event-stream");
        assertThat(first).startsWith("data:");
    }
}
