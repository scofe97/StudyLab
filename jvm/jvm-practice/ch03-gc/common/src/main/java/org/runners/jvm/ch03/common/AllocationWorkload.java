package org.runners.jvm.ch03.common;

import java.util.ArrayList;
import java.util.List;

// 책 §3.5~§3.6 비교용 공통 워크로드.
// - 짧은 수명 객체를 대량 생성 (Minor GC 빈도 자극)
// - 일부 객체는 살아남아 구세대로 승격 (Major GC 자극)
// - OOM 의도하지 않음. 시스템 위험 없음.
public final class AllocationWorkload {

    private static final int _1KB = 1024;

    private AllocationWorkload() {}

    public static void run() {
        run(50_000, 100);  // 기본: 50000 iter, 100 long-lived
    }

    public static void run(int iterations, int longLivedCount) {
        List<byte[]> longLived = new ArrayList<>(longLivedCount);
        for (int i = 0; i < iterations; i++) {
            byte[] shortLived = new byte[16 * _1KB];   // 16KB 짧은 수명
            shortLived[0] = (byte) i;                  // touch
            if (i % (iterations / longLivedCount) == 0) {
                longLived.add(new byte[64 * _1KB]);    // 64KB 살아남는 객체
            }
        }
        System.out.println("workload done. long-lived count = " + longLived.size());
    }
}
