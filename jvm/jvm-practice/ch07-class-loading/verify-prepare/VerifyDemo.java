// 바이트코드 검증 실습 — javap 로 StackMapTable 속성을 직접 본다.
//   분기(if)·반복(for)이 있으면 실행 경로가 갈린다. javac 가 각 분기점에서
//   피연산자 스택·지역 변수의 기대 타입을 StackMapTable 에 미리 적어둔다.
//   → JVM 검증은 처음부터 추론하지 않고 이 표와 대조만 하면 된다 (JDK 6+).
//
// 확인 명령:
//   javac VerifyDemo.java
//   javap -v -p VerifyDemo.class
//     → branchy() 메서드의 Code 속성 아래 "StackMapTable: number_of_entries = N"
//       그리고 frame 들(append/same/full_frame 등)이 보임 = 분기점별 타입 스냅숏
//   대조용: 분기 없는 noBranch() 에는 StackMapTable 이 없거나 항목이 적다.
public class VerifyDemo {

    // 분기·반복 있음 → StackMapTable 항목이 여러 개 생긴다
    public static int branchy(int n) {
        int sum = 0;
        for (int i = 0; i < n; i++) {   // 반복 → 백워드 점프, 분기점
            if (i % 2 == 0) {            // 분기 → 또 다른 frame
                sum += i;
            } else {
                sum -= i;
            }
        }
        return sum;
    }

    // 분기 없음 → 실행 경로가 하나라 StackMapTable 이 거의 없음 (대조용)
    public static int noBranch(int a, int b) {
        return a + b;
    }

    public static void main(String[] args) {
        System.out.println(branchy(5) + ", " + noBranch(2, 3));
    }
}
