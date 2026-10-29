// 예제 3 — 컴파일 타임 상수
// 기대: "hello world" 만 출력. "ConstClass init!" 은 안 찍힘
//   → static final 상수는 컴파일 때 값이 Test3 의 상수 풀로 복사돼,
//     런타임에 ConstClass 를 들여다볼 일이 없다.
public class Test3 {
    public static void main(String[] args) {
        // 상수를 참조
        System.out.println(ConstClass.HELLO_WORLD);
    }
}
