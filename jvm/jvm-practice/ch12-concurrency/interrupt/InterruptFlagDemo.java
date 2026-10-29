/**
 * isInterrupted() vs Thread.interrupted() — 플래그를 "유지"하느냐 "초기화"하느냐.
 *
 * 노트 03-01 §4-3 (스레드 생성과 생명주기) 의 SVG 대비를 출력으로 확인한다.
 *   - isInterrupted()      : 읽기만 함 → 두 번 읽어도 true, true
 *   - Thread.interrupted() : 읽고 false 로 clear → true, false
 *
 * 실행:
 *   JAVA_HOME=$(/usr/libexec/java_home -v 21)
 *   $JAVA_HOME/bin/javac InterruptFlagDemo.java
 *   $JAVA_HOME/bin/java InterruptFlagDemo
 */
public class InterruptFlagDemo {

    public static void main(String[] args) {
        // (1) 현재 스레드(main)의 인터럽트 플래그를 켠다
        Thread.currentThread().interrupt();
        System.out.println("interrupt() 호출 직후 플래그 = " + Thread.currentThread().isInterrupted());

        // (2) isInterrupted() — 인스턴스 메서드, 읽기만 함 (플래그 유지)
        Thread me = Thread.currentThread();
        System.out.println("\n[isInterrupted()] 읽고 둔다 — 멱등 조회");
        System.out.println("  1차 isInterrupted() = " + me.isInterrupted());
        System.out.println("  2차 isInterrupted() = " + me.isInterrupted() + "  (← 여전히 true, 유지됨)");

        // (3) Thread.interrupted() — static 메서드, 읽고 false 로 clear
        System.out.println("\n[Thread.interrupted()] 읽고 치운다 — 읽는 순간 비움");
        System.out.println("  1차 Thread.interrupted() = " + Thread.interrupted() + "  (← true 반환, 동시에 clear)");
        System.out.println("  2차 Thread.interrupted() = " + Thread.interrupted() + "  (← false, 이미 비워짐)");

        // (4) clear 된 뒤에는 isInterrupted() 도 false
        System.out.println("\nclear 후 isInterrupted() = " + me.isInterrupted() + "  (← 인터럽트 흔적 사라짐)");
    }
}
