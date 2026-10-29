import java.lang.reflect.Method;

// 전체를 묶는 입구. byte[] 를 받아 치환·적재·실행하고, HackSystem 버퍼에 모인 출력을 반환한다.
public class JavaClassExecuter {

    public static String execute(byte[] classByte) {
        // 1. 버퍼 초기화
        HackSystem.clearBuffer();
        // 2. 상수 풀의 java/lang/System → HackSystem 치환
        ClassModifier cm = new ClassModifier(classByte);
        byte[] modiBytes = cm.modifyUTF8Constant("java/lang/System", "HackSystem");
        // 3. 매번 새 로더 인스턴스로 적재 (핫스왑)
        HotSwapClassLoader loader = new HotSwapClassLoader();
        Class<?> clazz = loader.loadByte(modiBytes);
        // 4. 리플렉션으로 main(String[]) 실행
        try {
            Method method = clazz.getMethod("main", new Class[]{String[].class});
            method.invoke(null, new Object[]{new String[]{null}});
        } catch (Throwable e) {
            e.printStackTrace(HackSystem.out);   // 예외도 버퍼로
        }
        // 5. 버퍼에 쌓인 출력 반환
        return HackSystem.getBufferString();
    }

    // 데모: HelloWorld.class 를 읽어 실행하고, 출력이 "콘솔이 아니라 버퍼로" 왔음을 보여준다.
    public static void main(String[] args) throws Exception {
        byte[] bytes = java.nio.file.Files.readAllBytes(
                java.nio.file.Path.of("HelloWorld.class"));
        String result = execute(bytes);
        // 이 println 은 "진짜 System" — 여기에만 결과가 보이면, HelloWorld 의 출력이
        // 서버 콘솔로 새지 않고 버퍼를 거쳐 돌아온 것이다.
        System.out.println("=== 버퍼로 회수한 결과 ===");
        System.out.print(result);
        System.out.println("=== (위 두 줄이 HelloWorld 의 출력. 직접 콘솔이 아니라 execute 가 반환한 문자열) ===");
    }
}
