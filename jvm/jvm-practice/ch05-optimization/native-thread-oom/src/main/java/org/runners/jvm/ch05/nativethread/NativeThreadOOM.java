package org.runners.jvm.ch05.nativethread;

// 책 p.264 §5.2 — native thread OOM 재현
// VM 옵션: -Xmx256m (build.gradle.kts에서 박제)
// 기대 결과: java.lang.OutOfMemoryError: unable to create new native thread
//   힙은 멀쩡한데 *네이티브 메모리/OS 스레드 한도* 가 먼저 마른다.
// ⚠️ 시스템 스레드 한도를 건드린다. OS 가 불안정해질 수 있어 격리 환경에서만,
//    관찰 후 수동 종료. 운영/공용 머신에서 run 금지.
public final class NativeThreadOOM {

    public static void main(String[] args) {
        int count = 0;
        while (true) {
            // 끝없이 스레드를 만들어 네이티브 스택 자리를 소진 — 힙이 아니라
            // 네이티브가 먼저 마르는 지점에서 OutOfMemoryError 가 던져진다.
            new Thread(() -> {
                try {
                    Thread.sleep(Long.MAX_VALUE);   // 만든 스레드를 살려둬 자리를 점유
                } catch (InterruptedException ignored) {
                }
            }).start();
            count++;
            if (count % 100 == 0) {
                System.out.println("created threads = " + count);
            }
        }
    }
}
