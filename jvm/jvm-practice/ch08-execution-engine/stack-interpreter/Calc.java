// 실습 — 스택 기반 인스트럭션 셋: 모든 계산이 피연산자 스택을 밀고(push) 당겨(pop) 이뤄진다.
//
// 실행:
//   javac Calc.java
//   javap -c Calc            → 각 메서드 바이트코드. 레지스터 번호가 어디에도 없음을 확인.
//
// 확인 포인트:
//   1) calc(): (a+b)*c 가 iload/iadd/iload/imul/ireturn 으로 풀린다.
//      변수 초기화는 bipush/sipush(push) ↔ istore(pop) 왕복.
//   2) iadd 는 "b를 더하는" 명령이 아니다 — 스택에 이미 올라온 두 값을 pop 해 합을 push 한다.
//      그래서 더하기 직전에 iload 둘(피연산자 둘 올리기)이 반드시 선행한다.
//   3) addTwo(int,int): 같은 a+b 라도 지역변수가 인자로 들어오면 istore 왕복 없이 iload 부터 시작.
//      → istore 왕복은 "상수를 변수에 넣는" 초기화 때문이지 덧셈 자체 비용이 아님을 대조.
//   4) 어느 메서드에도 레지스터(eax 같은) 지정이 없음 = 스택 기반. x86 의 mov/add 2명령과 대조.
public class Calc {

    // (a + b) * c — 책 §8.5.3 예제. 변수 초기화(istore 왕복) + 산술(iadd/imul).
    public int calc() {
        int a = 100;
        int b = 200;
        int c = 300;
        return (a + b) * c;   // (100 + 200) * 300 = 90000
    }

    // 인자로 받은 두 값의 합 — istore 초기화 왕복 없이 iload_1, iload_2, iadd, ireturn 만.
    // calc() 의 a+b 부분과 대조해 "istore 왕복 = 초기화 비용"임을 본다.
    public int addTwo(int a, int b) {
        return a + b;
    }

    // 곱셈 한 번 — imul 도 iadd 와 똑같이 "스택의 두 값 pop, 곱 push".
    public int mul(int a, int b) {
        return a * b;
    }

    public static void main(String[] args) {
        Calc c = new Calc();
        System.out.println(c.calc());        // 90000
        System.out.println(c.addTwo(100, 200)); // 300
        System.out.println(c.mul(300, 300));    // 90000
    }
}
