/**
 * 가시성 버그 재현 — volatile 없는 공유 flag.
 *
 * 노트: ch05_efficient-concurrency/01-01 §3·§5 "양쪽 구멍"
 *
 * 기대: 메인 스레드가 1초 뒤 stop=true 로 바꾸면 worker 루프가 멈춰야 한다.
 * 실제: volatile 이 없으면 worker 가 영영 안 멈출 수 있다.
 *   - A 쪽 구멍: 메인이 바꾼 값을 worker 의 작업 메모리가 다시 안 읽음
 *   - B 쪽 구멍: JIT 가 while(!stop) 을 while(true) 로 끌어올려(hoist) 버림
 *
 * 실행:
 *   javac VisibilityBug.java
 *   java VisibilityBug        # "stopped" 가 안 찍히고 매달릴 수 있음 (Ctrl+C 로 종료)
 *
 * 환경에 따라 우연히 멈출 수도 있으니, 서버 JIT 를 강제하면 재현율이 올라간다:
 *   java -server -XX:-TieredCompilation VisibilityBug
 */
public class VisibilityBug {

    // ⚠️ volatile 이 빠진 평범한 공유 변수
    static boolean stop = false;

    public static void main(String[] args) throws InterruptedException {
        Thread worker = new Thread(() -> {
            long count = 0;
            // stop 이 true 가 되면 멈추길 기대한다
            while (!stop) {
                count++;
            }
            System.out.println("stopped, count=" + count);
        });

        worker.start();
        Thread.sleep(1000);   // worker 가 충분히 돌아 JIT 가 루프를 컴파일하도록 1초 대기

        stop = true;          // 메인 스레드: 이제 멈추라고 신호
        System.out.println("main: stop=true 로 설정함");

        worker.join(3000);    // 3초만 기다린다
        if (worker.isAlive()) {
            System.out.println("BUG: worker 가 3초 뒤에도 살아있다 (가시성 버그 재현)");
            System.exit(1);   // 매달린 worker 와 함께 강제 종료
        }
    }
}
