package org.runners.jvm.ch03.parallel;

import org.runners.jvm.ch03.common.AllocationWorkload;

// 책 §3.5.4 Parallel Scavenge + Parallel Old. 멀티스레드 STW, throughput 우선.
public final class ParallelGcDemo {

    public static void main(String[] args) {
        long t0 = System.currentTimeMillis();
        AllocationWorkload.run();
        long elapsed = System.currentTimeMillis() - t0;
        System.out.println("Parallel GC demo. elapsed = " + elapsed + " ms");
    }
}
