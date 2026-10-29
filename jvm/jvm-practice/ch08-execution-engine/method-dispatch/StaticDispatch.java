// 실습 ① 정적 디스패치(오버로딩) — 인자의 *정적 타입*으로 컴파일 때 버전이 정해진다.
//
// 실행:
//   javac StaticDispatch.java
//   java StaticDispatch                 → 둘 다 "hello, guy!" (정적 타입이 Human이라)
//   javap -c StaticDispatch             → main 의 invokevirtual #N // Method sayHello:(LHuman;)V
//                                         호출 대상이 sayHello(Human) 으로 *컴파일 때 박혀* 있음을 확인
//
//   포인트: man/woman 의 실제 타입은 Man/Woman 이지만, 바이트코드의 메서드 시그니처가
//          (Human) 으로 고정돼 있다. 컴파일러가 정적 타입을 보고 골랐다는 증거.
public class StaticDispatch {

    static abstract class Human {}
    static class Man extends Human {}
    static class Woman extends Human {}

    public void sayHello(Human guy) { System.out.println("hello, guy!"); }
    public void sayHello(Man guy)   { System.out.println("hello, gentleman!"); }
    public void sayHello(Woman guy) { System.out.println("hello, lady!"); }

    public static void main(String[] args) {
        // 정적 타입은 둘 다 Human, 실제 타입만 Man·Woman
        Human man = new Man();
        Human woman = new Woman();
        StaticDispatch sr = new StaticDispatch();
        sr.sayHello(man);     // → hello, guy!  (sayHello(Human))
        sr.sayHello(woman);   // → hello, guy!  (sayHello(Human))
    }
}
