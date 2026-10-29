package org.runners.jvm.ch03.g1;

import org.runners.jvm.ch03.common.AllocationWorkload;

// 책 §3.5.7 G1 GC. JDK 9+ 서버 디폴트. Region 기반 + 일시 정지 목표.
public final class G1GcDemo {

    public static void main(String[] args) {
        long t0 = System.currentTimeMillis();
        AllocationWorkload.run();

        long elapsed = System.currentTimeMillis() - t0;
        System.out.println("G1 GC demo. elapsed = " + elapsed + " ms");
    }
}
