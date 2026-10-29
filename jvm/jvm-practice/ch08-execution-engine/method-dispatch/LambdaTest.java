// 실습 ⑤ 람다는 익명 클래스가 아니다 — invokedynamic + LambdaMetafactory 로 컴파일된다.
//
// 실행:
//   javac LambdaTest.java
//   java LambdaTest                 → hello
//   javap -c -v LambdaTest          → main 에 invokedynamic, BootstrapMethods 에 LambdaMetafactory
//
//   확인 포인트:
//   1) main 의 바이트코드에 `invokedynamic #N, 0  // InvokeDynamic #0:run:()Ljava/lang/Runnable;`
//      → 람다 생성이 익명 클래스 new 가 아니라 invokedynamic 호출 지점으로 컴파일됨.
//   2) 파일 끝 BootstrapMethods 속성에:
//      `0: #.. REF_invokeStatic java/lang/invoke/LambdaMetafactory.metafactory(...)`
//      → 이 호출 지점의 부트스트랩 메서드가 LambdaMetafactory.metafactory 다.
//   3) 람다 본문은 별도 private 메서드(lambda$main$0 같은 이름)로 빠져 있음.
//
//   결론: "람다 = 익명 클래스"는 소스 관점의 비유일 뿐, 바이트코드는 익명 클래스 생성이 아니라
//         invokedynamic + LambdaMetafactory 가 런타임에 함수형 인터페이스 구현을 만들어 준다.
public class LambdaTest {
    public static void main(String[] args) {
        Runnable r = () -> System.out.println("hello");   // ← 이 줄이 invokedynamic 으로 컴파일됨
        r.run();
    }
}
