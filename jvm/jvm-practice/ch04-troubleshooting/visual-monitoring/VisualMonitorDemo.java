// VisualMonitorDemo — JConsole / IntelliJ Profiler 로 "관찰할" 대상 JVM.
// 문서 §2(JConsole 메모리 톱니 + 데드락 탐지)를 한 프로그램에 묶었다.
//
// 실행(메모리 톱니가 또렷하게 보이도록 SerialGC + 작은 힙):
//   javac VisualMonitorDemo.java
//   java -Xms100m -Xmx100m -XX:+UseSerialGC VisualMonitorDemo
//
// 그다음 다른 터미널에서:  jconsole   (또는 IntelliJ에서 이 프로세스에 attach)
//   - 메모리 탭: Eden 차오르다 Minor GC로 톱니처럼 급락하는 그래프
//   - 스레드 탭: [Detect Deadlock] 버튼 → 아래 두 스레드가 데드락으로 잡힘
import java.util.ArrayList;
import java.util.List;

public class VisualMonitorDemo {

    // 64KB 점유용 객체 — Eden 을 빠르게 채워 톱니 그래프를 만든다 (문서 §2의 OOMObject)
    static class OOMObject {
        public byte[] placeholder = new byte[64 * 1024];
    }

    // 살아있는 객체를 리스트에 쥐었다 풀며 GC 톱니를 눈으로 만든다.
    // 무한 반복하도록 바꿔, JConsole 을 붙일 시간을 충분히 준다.
    static void fillHeapForever() throws InterruptedException {
        while (true) {
            List<OOMObject> list = new ArrayList<>();
            for (int i = 0; i < 1000; i++) {
                Thread.sleep(20);          // 그래프가 천천히 그려지도록 텀
                list.add(new OOMObject());
            }
            list.clear();                  // 참조를 놓아 다음 GC 때 회수 → 톱니 급락
            System.gc();
        }
    }

    // --- 데드락: 두 스레드가 락 A,B 를 엇갈린 순서로 잡아 순환 대기 ---
    static final Object lockA = new Object();
    static final Object lockB = new Object();

    static void startDeadlock() {
        new Thread(() -> {
            synchronized (lockA) {
                sleep(100);
                synchronized (lockB) { }   // B 를 기다림 — 하지만 다른 스레드가 쥐고 있음
            }
        }, "deadlock-1").start();

        new Thread(() -> {
            synchronized (lockB) {
                sleep(100);
                synchronized (lockA) { }   // A 를 기다림 — 서로 마주 기다려 데드락
            }
        }, "deadlock-2").start();
    }

    static void sleep(long ms) {
        try { Thread.sleep(ms); } catch (InterruptedException e) { Thread.currentThread().interrupt(); }
    }

    public static void main(String[] args) throws InterruptedException {
        System.out.println("PID ready. attach jconsole / IntelliJ now.");
        startDeadlock();                   // 데드락 스레드 2개 (스레드 탭에서 관찰)
        fillHeapForever();                 // 메모리 톱니 (메모리 탭에서 관찰)
    }
}
