package org.runners.jvm.ch03.zgc;

import org.runners.jvm.ch03.common.AllocationWorkload;

// 책 §3.6.2 ZGC. JDK 15+ stable, JDK 21+ Generational stable.
// 컬러 포인터 + Load Barrier 로 일시 정지 < 1ms 목표.
public final class ZgcDemo {

    public static void main(String[] args) {
        long t0 = System.currentTimeMillis();
        AllocationWorkload.run();
        long elapsed = System.currentTimeMillis() - t0;
        System.out.println("ZGC demo. elapsed = " + elapsed + " ms");
    }
}
