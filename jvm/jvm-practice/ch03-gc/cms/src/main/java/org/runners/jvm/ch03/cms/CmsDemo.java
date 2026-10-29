package org.runners.jvm.ch03.cms;

import org.runners.jvm.ch03.common.AllocationWorkload;

// 책 §3.5.6 CMS. JDK 9 deprecated, JDK 14 제거.
// JDK 21에서는 -XX:+UseConcMarkSweepGC 옵션이 더 이상 받아들여지지 않는다.
// 본 모듈은 *역사적 학습용* 코드 박제. 실행은 디폴트 GC(G1) 으로 떨어진다.
public final class CmsDemo {

    public static void main(String[] args) {
        System.out.println("CMS는 JDK 14에서 제거됨. 이 데모는 디폴트 GC(G1)로 동작한다.");
        System.out.println("CMS의 동작은 책 §3.5.6의 4단계 텍스트로 학습한다:");
        System.out.println("  1. Initial Mark (STW)");
        System.out.println("  2. Concurrent Mark");
        System.out.println("  3. Remark (STW)");
        System.out.println("  4. Concurrent Sweep");
        long t0 = System.currentTimeMillis();
        AllocationWorkload.run();
        long elapsed = System.currentTimeMillis() - t0;
        System.out.println("elapsed = " + elapsed + " ms (실제 GC = G1)");
    }
}
