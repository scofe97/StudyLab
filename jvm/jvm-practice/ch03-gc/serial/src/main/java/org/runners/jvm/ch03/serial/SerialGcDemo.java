package org.runners.jvm.ch03.serial;

import org.runners.jvm.ch03.common.AllocationWorkload;

// 책 §3.5.1 Serial GC. 단일 스레드 STW + 마크-카피.
// GC 로그는 build/gc.log 에 저장됨.
public final class SerialGcDemo {

    public static void main(String[] args) {
        long t0 = System.currentTimeMillis();
        AllocationWorkload.run();
        long elapsed = System.currentTimeMillis() - t0;
        System.out.println("Serial GC demo. elapsed = " + elapsed + " ms");
        System.out.println("GC log: build/gc.log");
    }
}
