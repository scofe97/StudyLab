package org.runners.jvm.ch04.hsdis;

// 책 p.244 §4.4 — HSDIS 디스어셈블 대상 (단순 산술 메서드)
// VM 옵션: -XX:+UnlockDiagnosticVMOptions -XX:+PrintAssembly -Xcomp
//          -XX:CompileCommand=compileonly,*Bar.sum (build.gradle.kts에서 박제)
// 기대 결과: sum() 의 JIT 컴파일 기계어(어셈블리)가 표준 출력으로 나온다.
//   인스턴스 필드 a + 정적 필드 b + 인자 c 의 덧셈이 어떤 네이티브 명령으로
//   떨어지는지 확인한다.
public final class Bar {
    int a = 1;
    static int b = 2;

    public int sum(int c) {
        return a + b + c;
    }

    public static void main(String[] args) {
        new Bar().sum(3);
    }
}
