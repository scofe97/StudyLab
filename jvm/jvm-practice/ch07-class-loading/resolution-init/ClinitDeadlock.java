// 실습 ② clinit 데드락 함정 — <clinit> 안 무한 루프가 초기화 락을 영영 안 놓는다.
//   두 스레드가 동시에 DeadLoopClass 를 초기화 시도 →
//   한 스레드만 <clinit>에 진입해 while(true) 에 갇히고 락을 못 놓음 →
//   다른 스레드는 초기화 락을 기다리며 BLOCKED. 둘 다 "run over" 못 찍음.
//
//   일반 무한 루프와 다른 점: 한 스레드만 루프를 돌고(RUNNABLE) 나머지는
//   조용히 대기(BLOCKED)라 원인 찾기가 까다롭다.
//
// 실행: javac ClinitDeadlock.java && java ClinitDeadlock
//   → 두 "start" 는 찍히지만 "run over" 는 안 찍히고 멈춤(Ctrl+C 로 종료)
//   진단: 다른 터미널에서  jps -l  →  jstack <pid>
//     한 스레드 RUNNABLE(<clinit> 루프) + 다른 스레드 BLOCKED(초기화 락 대기) 관찰
class DeadLoopClass {
    static {
        // 의도적 무한 루프 — 한 스레드가 여기서 멈추면 초기화 락을 놓지 못함
        if (true) {
            System.out.println(Thread.currentThread() + " init DeadLoopClass");
            while (true) { }   // <clinit> 이 끝나지 않음
        }
    }
}

public class ClinitDeadlock {
    public static void main(String[] args) {
        Runnable script = () -> {
            System.out.println(Thread.currentThread() + " start");
            // 두 스레드가 같은 클래스를 동시에 초기화 시도
            DeadLoopClass dlc = new DeadLoopClass();
            System.out.println(Thread.currentThread() + " run over");
        };
        new Thread(script).start();
        new Thread(script).start();
    }
}
