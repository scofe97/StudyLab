// 예제 2 — 배열 선언
// 기대: 아무것도 안 찍힘. "SuperClass init!" 도 안 나옴
//   → new SuperClass[10] 은 SuperClass 초기화가 아니라
//     JVM이 배열 클래스 [LSuperClass 를 만드는 동작(anewarray)이다.
public class Test2 {
    public static void main(String[] args) {
        // SuperClass 의 배열을 선언만 함 — 인스턴스 생성 아님
        SuperClass[] array = new SuperClass[10];
        System.out.println("배열 선언 완료 (위에 init! 이 안 나왔으면 통과)");
    }
}
