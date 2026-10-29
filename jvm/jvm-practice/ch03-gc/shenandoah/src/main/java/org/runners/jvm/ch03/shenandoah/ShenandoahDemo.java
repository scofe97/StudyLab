package org.runners.jvm.ch03.shenandoah;

import org.runners.jvm.ch03.common.AllocationWorkload;

// 책 §3.6.1 Shenandoah GC. Red Hat 제작.
// Forwarding Pointer + Read Barrier 로 동시 정리 가능.
// 주의: Oracle JDK에는 미포함. Temurin·Corretto 등 OpenJDK 빌드 사용 필요.
public final class ShenandoahDemo {

    public static void main(String[] args) {
        long t0 = System.currentTimeMillis();
        AllocationWorkload.run();
        long elapsed = System.currentTimeMillis() - t0;
        System.out.println("Shenandoah demo. elapsed = " + elapsed + " ms");
    }
}
