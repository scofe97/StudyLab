/**
 * 가시성 버그 수정 — volatile 한 글자로 양쪽 구멍을 함께 막는다.
 *
 * 노트: ch05_efficient-concurrency/01-01 §5 "volatile 은 둘 다 강제"
 *
 * volatile 이 하는 일:
 *   - 쓰기: stop=true 가 즉시 메인 메모리에 반영 (A 쪽 store·write 강제)
 *   - 읽기: while(!stop) 이 매번 메인 메모리에서 새로 읽음 (B 쪽 read·load 강제)
 *           → JIT 의 while(true) hoist 도 막힌다
 *ㅊ
 * 실행:
 *   javac VisibilityFixed.java
 *   java VisibilityFixed       # 1초 뒤 "stopped" 가 반드시 찍히고 정상 종료
 */
public class VisibilityFixed {

    // ✅ volatile 한 단어가 차이의 전부다 — VisibilityBug 와 이 줄만 다르다
    static volatile boolean stop = false;

    public static void main(String[] args) throws InterruptedException {
        Thread worker = new Thread(() -> {
            long count = 0;
            while (!stop) {
                count++;
            }
            System.out.println("stopped, count=" + count);
        });

        worker.start();
        Thread.sleep(1000);

        stop = true;
        System.out.println("main: stop=true 로 설정함");

        worker.join(3000);
        if (worker.isAlive()) {
            System.out.println("예상 밖: volatile 인데도 안 멈췄다");
            System.exit(1);
        } else {
            System.out.println("OK: worker 가 정상적으로 멈췄다 (volatile 이 가시성 보장)");
        }
    }
}
