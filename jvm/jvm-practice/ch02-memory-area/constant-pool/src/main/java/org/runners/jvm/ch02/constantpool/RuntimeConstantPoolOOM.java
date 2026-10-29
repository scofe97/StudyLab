package org.runners.jvm.ch02.constantpool;

import java.util.ArrayList;
import java.util.List;

// 책 p.87 §2.4.3 런타임 상수 풀 OOM
// JDK 7+ 변경 사유: String.intern()이 추가하는 문자열의 실체가 자바 힙으로 이동했다.
//   책은 JDK 6 기준 -XX:MaxPermSize=6m 으로 PermGen OOM을 유도하지만,
//   JDK 21에서는 -Xmx 한계가 먼저 부딪힌다.
// VM 옵션: -Xms10m -Xmx10m
// 기대 결과: java.lang.OutOfMemoryError: Java heap space
public final class RuntimeConstantPoolOOM {

    public static void main(String[] args) {
        List<String> list = new ArrayList<>();
        long i = 0;
        while (true) {
            list.add(String.valueOf(i++).intern());
        }
    }
}
