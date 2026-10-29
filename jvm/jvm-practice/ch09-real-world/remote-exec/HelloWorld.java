// 실습 — 04-03 상수 풀 치환의 토대: System 참조가 상수 풀에 문자열로 박힌다.
//
// 04-03 의 HackSystem 트릭은 "이 .class 의 상수 풀에서 java/lang/System 문자열을
// HackSystem 으로 바꾸면 System 참조가 통째로 바뀐다"는 것이다. 그 전제 —
// "System 참조가 정말 상수 풀에 문자열로 들어있는가" — 를 javap 로 직접 확인한다.
//
// 실행:
//   javac HelloWorld.java
//   javap -v HelloWorld        → Constant pool 섹션에서 다음을 찾는다:
//     1) Utf8  java/lang/System              ← 04-03 이 치환하는 그 문자열
//     2) Class #N  // java/lang/System
//     3) Fieldref ... java/lang/System.out:Ljava/io/PrintStream;
//   그리고 main 의 Code 에서:
//     getstatic #N   // Field java/lang/System.out  ← 명령은 문자열이 아니라 인덱스(#N)를 가리킴
//
// 확인 포인트:
//   - System 을 두 번 호출(아래)해도 상수 풀의 java/lang/System Utf8 은 "한 항목"뿐이다.
//     → 그 한 곳만 바꾸면 두 호출 모두 HackSystem 으로 바뀐다(§2 보강의 "인덱스 참조" 근거).
//   - 바이트코드 getstatic/invokevirtual 은 #N 인덱스를 들고 있지 문자열을 직접 들지 않는다.
public class HelloWorld {
    public static void main(String[] args) {
        System.out.println("hello");

        System.out.println("world");   // System 두 번째 참조 — 상수 풀 항목은 공유됨
    }
}
