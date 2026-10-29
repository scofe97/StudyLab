// 노트 §3 예제 1·2 공용 — 부모 클래스
// static 블록이 찍히면 = SuperClass 가 초기화됐다는 관측 지점
class SuperClass {
    static {
        System.out.println("SuperClass init!");
    }
    public static int value = 123;   // static 필드는 부모에 선언
}

// 예제 1 공용 — 자식 클래스
class SubClass extends SuperClass {
    static {
        System.out.println("SubClass init!");
    }
}
