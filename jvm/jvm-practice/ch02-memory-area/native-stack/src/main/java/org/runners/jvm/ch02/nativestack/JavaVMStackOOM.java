package org.runners.jvm.ch02.nativestack;

// 책 p.85 §2.4.2 스레드 폭주에 의한 OOM
// VM 옵션: -Xss2m (스레드당 큰 스택을 잡아 빠르게 한계 도달)
// 기대 결과: java.lang.OutOfMemoryError: unable to create native thread
//
// 주의: 이 코드는 OS의 스레드 수 한계까지 스레드를 만들어 시스템 응답성을 떨어뜨릴 수 있다.
//      격리된 컨테이너나 ulimit -u 로 제한된 환경에서 실행한다.
//      운영 머신에서 실행하면 다른 프로세스가 영향을 받을 수 있다.
public final class JavaVMStackOOM {

    private void dontStop() {
        while (true) {
            // 스레드가 종료되지 않도록 무한 루프
        }
    }

    public void stackLeakByThread() {
        while (true) {
            new Thread(this::dontStop).start();
        }
    }

    public static void main(String[] args) {
        new JavaVMStackOOM().stackLeakByThread();
    }
}
