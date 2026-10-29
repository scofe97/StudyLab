// 실습 ① 부모 먼저 — 자식 초기화 전에 부모 <clinit>이 먼저 돈다.
//   Parent: static A=1, static{ A=2 }  →  Parent <clinit> 후 A=2
//   Sub:    static B=A
//   Sub.B 를 읽으면 Sub 초기화가 필요한데, 그 전에 Parent 가 먼저 초기화돼 A=2.
//   그래서 B = A = 2  (1 이 아니다)
//
// 실행: javac ParentFirst.java && java ParentFirst   → 출력: 2
class Parent {
    static int A = 1;
    static { A = 2; }        // Parent <clinit>: A = 2
}

class Sub extends Parent {
    static int B = A;        // Sub <clinit>: B = A (이때 A 는 이미 2)
}

public class ParentFirst {
    public static void main(String[] args) {
        // Sub.B 를 읽는 순간 Parent 초기화가 먼저 일어난다
        System.out.println(Sub.B);   // 기대: 2
    }
}
