import java.io.ByteArrayOutputStream;
import java.io.PrintStream;

// System 을 본뜬 대체 클래스. 치환된 클래스가 "System.out" 대신 "HackSystem.out" 을 부르게 되면,
// 그 출력이 서버 콘솔이 아니라 이 buffer(ByteArrayOutputStream) 로 모인다.
// 클래스 이름이 정확히 "HackSystem" 이어야 한다 — ClassModifier 가 java/lang/System → HackSystem 으로
// 치환하므로, 패키지 없는 최상위 클래스 HackSystem 이 그 자리를 받는다.
public class HackSystem {

    // System 의 in 은 그대로 위임(여기선 미사용)
    public static final java.io.InputStream in = System.in;

    private static final ByteArrayOutputStream buffer = new ByteArrayOutputStream();

    // 치환된 클래스가 System.out → HackSystem.out 으로 부를 때 닿는 곳. 버퍼로 연결.
    public static final PrintStream out = new PrintStream(buffer);
    public static final PrintStream err = out;   // err 도 같은 버퍼로

    public static void clearBuffer() {
        buffer.reset();
    }

    public static String getBufferString() {
        return buffer.toString();
    }

    // System 의 자주 쓰는 정적 메서드들도 본떠 위임 — 치환된 클래스가 호출할 수 있으므로.
    public static void exit(int status)            { /* 격리: 실제 종료 막음 */ }
    public static void gc()                        { System.gc(); }
    public static long currentTimeMillis()         { return System.currentTimeMillis(); }
    public static long nanoTime()                  { return System.nanoTime(); }
    public static String getProperty(String key)   { return System.getProperty(key); }
    public static void arraycopy(Object src, int sp, Object dst, int dp, int len) {
        System.arraycopy(src, sp, dst, dp, len);
    }
}
