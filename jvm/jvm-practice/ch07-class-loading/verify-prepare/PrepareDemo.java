// 준비 단계 실습 — javap 로 ConstantValue 속성을 직접 본다.
//   일반 static int value = 123  → ConstantValue 없음. 대입은 <clinit> 의 putstatic.
//                                   준비 직후 값은 0, 초기화 때 비로소 123.
//   static final int CONST = 456 → ConstantValue 속성 붙음. 준비 단계에서 즉시 456.
//
// 확인 명령:
//   javac PrepareDemo.java
//   javap -v -p PrepareDemo.class
//     → CONST 필드 아래 "ConstantValue: int 456" 이 보임 (value 에는 없음)
//     → static {} = <clinit> 안에 "putstatic ... value" 가 보임 (value=123 대입이 여기 모임)
public class PrepareDemo {

    // 일반 static — ConstantValue 없음, 준비 때 0, <clinit>에서 123
    public static int value = 123;

    // static final 컴파일 타임 상수 — ConstantValue 속성, 준비 때 즉시 456
    public static final int CONST = 456;

    // 참고: 런타임에 값이 정해지는 final 은 ConstantValue 가 안 붙는다 (복사 불가)
    public static final Object RUNTIME_CONST = new Object();

    public static void main(String[] args) {
        System.out.println("value = " + value + ", CONST = " + CONST);
    }
}
