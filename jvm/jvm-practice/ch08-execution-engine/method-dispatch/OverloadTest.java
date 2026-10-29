// 실습 ③ 같은 구조에서 오버라이딩 대신 오버로딩만 썼을 때 — 동적처럼 흘러갈까?
//
// DynamicDispatch 와 같은 Human/Man/Woman 계층이지만, sayHello 를 자식이 오버라이딩하지 않고
// OverloadTest 한 클래스 안에 *파라미터 타입만 다른* 3버전으로 둔다(오버로딩).
//
// 실행:
//   javac OverloadTest.java
//   java OverloadTest
//   javap -c OverloadTest | grep -A20 "public static void main"
//
//   비교 포인트:
//   - DynamicDispatch 는 man=new Man()/new Woman() 에 따라 출력이 갈렸다(실제 타입 따라감).
//   - OverloadTest 는? man 의 *선언 타입*이 Human 이라, 실제 타입을 바꿔도 출력이 안 바뀐다.
//     javap 의 invokevirtual 시그니처가 (Human) 으로 박혀 있음 = 컴파일 때 선언 타입으로 확정.
public class OverloadTest {

    static abstract class Human {}
    static class Man extends Human {}
    static class Woman extends Human {}

    // 오버로딩: 이름 같고 파라미터 타입만 다른 3버전
    void sayHello(Human g) { System.out.println("human"); }
    void sayHello(Man g)   { System.out.println("man"); }
    void sayHello(Woman g) { System.out.println("woman"); }

    public static void main(String[] args) {
        OverloadTest t = new OverloadTest();

        Human man = new Man();
        t.sayHello(man);        // 무엇이 출력될까? (선언 타입 Human vs 실제 타입 Man)

        man = new Woman();      // 같은 변수, 실제 타입만 Woman 으로 교체
        t.sayHello(man);        // 출력이 바뀔까? (DynamicDispatch 는 바뀌었음)

        // 대조: 실제 타입을 컴파일러가 아는 경우엔 그 시그니처가 잡힌다
        Man realMan = new Man();
        t.sayHello(realMan);    // 이건 sayHello(Man) — 선언 타입이 Man 이므로
    }
}
