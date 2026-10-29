// 노트 §3 예제 3 공용 — 컴파일 타임 상수를 담은 클래스
class ConstClass {
    static {
        System.out.println("ConstClass init!");
    }
    // static final 컴파일 타임 상수
    public static final String HELLO_WORLD = "hello world";
}
