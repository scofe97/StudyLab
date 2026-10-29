// 실습 ① Code 속성 — javap -v 로 max_locals / max_stack 이 .class 에 박힌 것을 본다.
//   메서드의 변수 개수·연산 깊이는 코드 구조로 정해지므로 컴파일 때 확정된다.
//
// 실행: javac FrameSize.java && javap -v FrameSize.class
//   → 각 메서드 Code: 줄에 "stack=N, locals=M" 이 보임 (= max_stack, max_locals)
//     add(int,int)  : locals 작음(this 없음=static, 파라미터 2) · stack 2 (a push,b push)
//     instanceAdd   : locals 에 this 포함(slot0)
//     withLong      : long 이 slot 2칸 → locals 가 더 큼
public class FrameSize {

    // static 2-인자 덧셈 — this 없음. stack 깊이 2(a,b 동시 적재)
    static int add(int a, int b) {
        return a + b;
    }

    // 인스턴스 메서드 — slot0 = this 라 locals 가 한 칸 더
    int instanceAdd(int a, int b) {
        return a + b;
    }

    // long 은 slot 2칸 사용 → locals 가 커진다
    static long withLong(long x, int y) {
        long z = x + y;
        return z;
    }

    public static void main(String[] args) {
        System.out.println(add(1, 2));
        System.out.println(new FrameSize().instanceAdd(3, 4));
        System.out.println(withLong(5L, 6));
    }
}
