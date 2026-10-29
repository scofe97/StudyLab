// LeakDemo — 진단 도구(jps/jstat/jmap)로 "떠볼" 대상 JVM.
// 목적: Old 영역에 회수되지 않는 객체를 의도적으로 쌓아 메모리 누수를 재현한다.
//   → jstat -gcutil 로 O(Old 사용률)가 우상향하고 FGC 후에도 안 내려가는 패턴을 관찰하고,
//     jmap -histo / -dump 로 "범인 객체"(여기선 byte[])를 확인하는 게 실습의 핵심.
//
// 실행: java -Xmx64m -Xlog:gc LeakDemo
//   -Xmx64m  : 힙을 64MB로 좁혀 누수가 빨리 드러나게 한다(곧 OOM까지 감).
//   -Xlog:gc : GC가 언제 도는지 콘솔로 같이 본다(jstat 관찰과 대조용).
import java.util.ArrayList;
import java.util.List;

public class LeakDemo {

    // static 컬렉션에 계속 add 하기만 하고 remove 하지 않는다.
    // GC 루트(static 필드)에서 도달 가능하므로, Full GC 가 돌아도 회수되지 않는다 —
    // 이것이 "부하 증가로 인한 정상 점유"가 아니라 "누수"인 이유다.
    private static final List<byte[]> LEAK = new ArrayList<>();

    public static void main(String[] args) throws InterruptedException {
        // 진단 도구가 jps 로 이 프로세스를 붙잡을 시간을 벌기 위해 잠깐 대기.
        System.out.println("PID ready. attach jps/jstat/jmap now. start in 10s...");
        Thread.sleep(10_000);

        long round = 0;
        while (true) {
            // 한 번에 약 1MB(1024 * 1KB)씩 누적. 64MB 힙이면 수십 라운드 안에 Old 가 찬다.
            for (int i = 0; i < 1024; i++) {
                LEAK.add(new byte[1024]); // 1KB 블록
            }
            round++;
            // 250ms 간격 jstat 관찰과 보조를 맞추기 위해 천천히 증가시킨다.
            Thread.sleep(200);
            if (round % 5 == 0) {
                System.out.println("leaked ~" + round + " MB (LEAK size=" + LEAK.size() + ")");
            }
        }
    }
}
