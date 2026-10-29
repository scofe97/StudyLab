// 실습 ② 동적 디스패치(오버라이딩) — 수신자의 *실제 타입*으로 실행 중에 버전이 정해진다.
//
// 실행:
//   javac DynamicDispatch.java
//   java DynamicDispatch                → man say hello / woman say hello / woman say hello
//   javap -c DynamicDispatch            → main 의 invokevirtual #N // Method Human.sayHello:()V
//                                         바이트코드는 Human.sayHello 로 같은데, 실행 결과가 갈린다.
//
//   포인트: 같은 invokevirtual 한 줄인데도 man=new Man()/new Woman() 에 따라 다른 메서드가 불린다.
//          컴파일러는 Human.sayHello 만 적어두고, 실제 어느 구현을 부를지는 JVM 이 실행 중에
//          수신자의 실제 타입을 보고 결정(자식→부모 탐색)한다는 증거.
public class DynamicDispatch {

    static abstract class Human {
        protected abstract void sayHello();
    }
    static class Man extends Human {
        @Override protected void sayHello() { System.out.println("man say hello"); }
    }
    static class Woman extends Human {
        @Override protected void sayHello() { System.out.println("woman say hello"); }
    }

    public static void main(String[] args) {
        Human man = new Man();
        Human woman = new Woman();
        man.sayHello();      // → man say hello   (실제 타입 Man)
        woman.sayHello();    // → woman say hello (실제 타입 Woman)

        man = new Woman();   // 같은 변수, 실제 타입만 Woman 으로 교체
        man.sayHello();      // → woman say hello (실제 타입을 따라감)
    }
}
