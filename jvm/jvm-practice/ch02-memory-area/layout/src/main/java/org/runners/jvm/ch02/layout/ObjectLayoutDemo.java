package org.runners.jvm.ch02.layout;

import org.openjdk.jol.info.ClassLayout;
import org.openjdk.jol.vm.VM;

// 책 §2.3.2 객체의 메모리 레이아웃 (p.73) — JOL 로 헤더·인스턴스 데이터·패딩 출력
// VM 옵션: 기본
// 출력 해석:
//   - 헤더가 12바이트면 압축 OOP 켜짐 (-XX:+UseCompressedOops, JDK 21 기본)
//   - 헤더가 16바이트면 압축 OOP 꺼짐
public final class ObjectLayoutDemo {

    public static void main(String[] args) {
        System.out.println(VM.current().details());
        System.out.println();


        System.out.println("=== Object ===");
        System.out.println(ClassLayout.parseClass(Object.class).toPrintable());

        System.out.println("=== Sample (int + boolean + reference) ===");
        System.out.println(ClassLayout.parseClass(Sample.class).toPrintable());

        System.out.println("=== int[16] ===");
        System.out.println(ClassLayout.parseInstance(new int[16]).toPrintable());
    }

    static class Sample {
        int i;
        boolean flag;
        Object ref;
    }
}
