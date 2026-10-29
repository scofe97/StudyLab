// 실습 ② slot 재사용 GC 함정 — scope 끝나도 slot 이 참조를 들면 회수 안 됨.
//   3가지 버전을 인자로 토글해 GC 로그로 회수 여부를 비교한다.
//
// 실행(GC 로그 보며):
//   javac SlotGc.java
//   java -Xlog:gc -Xmx256m SlotGc keep    ← placeholder slot 안 덮음 → System.gc() 후에도 64MB 살아있음
//   java -Xlog:gc -Xmx256m SlotGc reuse   ← int a=0 으로 slot 덮음 → 회수됨
//   java -Xlog:gc -Xmx256m SlotGc null    ← placeholder=null 로 끊음 → 회수됨
//
//   gc 로그의 "Pause Full ... 65M->1M" 처럼 회수되면 줄어들고, "65M->65M" 면 안 줄어든 것.
public class SlotGc {

    public static void main(String[] args) {
        String mode = args.length > 0 ? args[0] : "keep";

        if (mode.equals("keep")) {
            {
                byte[] placeholder = new byte[64 * 1024 * 1024];   // 64MB
                System.out.println("placeholder 생성: " + placeholder.length);
            }
            // scope 끝났지만 slot 이 아직 그 참조를 들고 있음 (재사용/null 없음)
            System.gc();   // 회수 안 됨 → gc 로그 65M->65M 근처
            System.out.println("[keep] System.gc() 호출 — 회수 안 됨 예상");

        } else if (mode.equals("reuse")) {
            {
                byte[] placeholder = new byte[64 * 1024 * 1024];
                System.out.println("placeholder 생성: " + placeholder.length);
            }
            int a = 0;     // 새 변수 a 가 placeholder 의 slot 을 재사용 → 참조 끊김
            System.gc();   // 회수됨 → gc 로그 65M->1M 근처
            System.out.println("[reuse] int a=" + a + " 로 slot 덮음 — 회수됨 예상");

        } else {  // null
            byte[] placeholder = new byte[64 * 1024 * 1024];
            System.out.println("placeholder 생성: " + placeholder.length);
            placeholder = null;   // 명시적으로 참조 끊기
            System.gc();          // 회수됨
            System.out.println("[null] placeholder=null 로 끊음 — 회수됨 예상");
        }
    }
}
