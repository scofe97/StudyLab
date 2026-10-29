package org.runners.jvm.ch04.monitoring;

// 책 p.226 §4.3.2 — JConsole 스레드 탭 / 데드락 탐지 관찰용
// VM 옵션: -Xms100m -Xmx100m -XX:+UseSerialGC -Dcom.sun.management.jmxremote (build.gradle.kts에서 박제)
// 관찰 방법: jconsole 실행 → 이 프로세스 attach → 스레드 탭에서
//   (1) testBusyThread 가 RUNNABLE 로 CPU 점유
//   (2) Detect Deadlock 버튼으로 testLockThread 쌍이 데드락으로 잡힘
// ⚠️ 이 프로그램은 의도적으로 멈추지 않는다(무한 루프 + wait). run 으로 띄워 관찰 후 수동 종료.
public final class ThreadMonitoringTest {

    // 무한 루프로 CPU 를 점유 — 스레드 탭에서 RUNNABLE 상태로 보인다
    public static void createBusyThread() {
        Thread thread = new Thread(() -> {
            while (true) {
                // 의도적 busy-wait: JConsole 에서 CPU 점유 스레드 식별 실습
            }
        }, "testBusyThread");
        thread.start();
    }

    // 락을 쥔 채 wait — 깨워줄 스레드가 없어 영영 대기(데드락 유발 시드)
    public static void createLockThread(final Object lock) {
        Thread thread = new Thread(() -> {
            synchronized (lock) {
                try {
                    lock.wait();
                } catch (InterruptedException e) {
                    e.printStackTrace();
                }
            }
        }, "testLockThread");
        thread.start();
    }

    public static void main(String[] args) throws Exception {
        // CPU 점유 스레드를 띄워 JConsole 스레드 탭에서 식별 실습
        createBusyThread();

        // 같은 락을 두 스레드가 쥐려다 마주 대기 → Detect Deadlock 으로 잡기
        Object lock = new Object();
        createLockThread(lock);
        createLockThread(lock);
    }
}
