// 예제 1 — 자식을 통한 부모의 static 필드 접근
// 기대: "SuperClass init!" + "123" 만 출력. "SubClass init!" 은 안 찍힘
//   → static 필드는 그 필드를 선언한 클래스(SuperClass)만 초기화한다.
public class Test1 {
    public static void main(String[] args) {
        // SubClass 를 통해 부모 필드 value 에 접근
        System.out.println(SubClass.value);
    }
}
