package org.runners.jvm.ch03.allocation;

// 책 §3.8.1 EdenAllocation — 객체는 Eden 에 우선 할당된다.
// VM 옵션: -Xms20m -Xmx20m -Xmn10m -XX:SurvivorRatio=8 -XX:+UseParallelGC
public final class TestAllocation {

    private static final int _1MB = 1024 * 1024;

    public static void main(String[] args) {
        byte[] a1 = new byte[2 * _1MB];
        byte[] a2 = new byte[2 * _1MB];
        byte[] a3 = new byte[2 * _1MB];
        byte[] a4 = new byte[2 * _1MB];
        byte[] a5 = new byte[_1MB];

        System.out.println("TestAllocation done. GC 로그는 build/gc.log 에서 확인");
        // a1~a4 를 살아 있게 유지
        System.out.println("alive: " + (a1.length + a2.length + a3.length + a4.length + a5.length) + " bytes");
    }
}
