package tpr;

import static org.assertj.core.api.Assertions.assertThat;

import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import org.junit.jupiter.api.Test;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.test.web.server.LocalServerPort;

@SpringBootTest(webEnvironment = SpringBootTest.WebEnvironment.RANDOM_PORT)
class LoadControllerTest {

    @LocalServerPort
    int port;

    private HttpResponse<byte[]> get(String path) throws Exception {
        HttpRequest req = HttpRequest.newBuilder(URI.create("http://localhost:" + port + path)).build();
        return HttpClient.newHttpClient().send(req, HttpResponse.BodyHandlers.ofByteArray());
    }

    @Test
    void small은_짧은_JSON을_돌려준다() throws Exception {
        // Given — (서버는 랜덤 포트로 기동됨)

        // When
        HttpResponse<byte[]> res = get("/small");

        // Then
        assertThat(res.statusCode()).isEqualTo(200);
        assertThat(new String(res.body())).isEqualTo("{\"ok\":true}");
    }

    @Test
    void slow는_요청한_시간만큼_기다린다() throws Exception {
        // Given
        long start = System.nanoTime();

        // When
        HttpResponse<byte[]> res = get("/slow?ms=100");

        // Then
        long elapsedMs = (System.nanoTime() - start) / 1_000_000;
        assertThat(res.statusCode()).isEqualTo(200);
        assertThat(elapsedMs).isGreaterThanOrEqualTo(100);
    }

    @Test
    void big은_kb만큼_본문을_보낸다() throws Exception {
        // Given — 4KB 요청

        // When
        HttpResponse<byte[]> res = get("/big?kb=4");

        // Then
        assertThat(res.statusCode()).isEqualTo(200);
        assertThat(res.body()).hasSize(4 * 1024);
    }
}
