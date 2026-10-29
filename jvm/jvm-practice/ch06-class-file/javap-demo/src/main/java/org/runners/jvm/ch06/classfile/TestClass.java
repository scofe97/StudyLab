package org.runners.jvm.ch06.classfile;

// 책 §6.3 Code 6-1 류 — javap 로 클래스 파일 구조를 역어보기 위한 예제
// 관찰 방법(컴파일 후):
//   javap -v  build/classes/java/main/org/runners/jvm/ch06/classfile/TestClass.class
//     → magic(cafebabe)·버전·상수 풀(Constant pool)·access_flags·필드·메서드·속성 전체
//   javap -c  ...TestClass.class
//     → inc()/getM() 의 바이트코드(iload·iadd·getfield·invokevirtual 등)
// 정적 필드 + 인스턴스 필드 + 메서드를 한 개씩 둬서 상수 풀 참조가 골고루 나오게 했다.
public class TestClass {

    private int m;            // 인스턴스 필드 — getfield/putfield 로 접근
    private static int n = 7; // 정적 필드 — getstatic/putstatic 로 접근

    public int inc() {
        // m + n 덧셈 — javap -c 에서 iload·getfield·getstatic·iadd 로 떨어진다
        return m + n;
    }

    public int getM() {
        return m;
    }
}
