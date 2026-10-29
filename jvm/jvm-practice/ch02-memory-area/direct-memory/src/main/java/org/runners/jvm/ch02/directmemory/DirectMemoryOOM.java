package org.runners.jvm.ch02.directmemory;

import java.lang.reflect.Field;
import sun.misc.Unsafe;

// 책 p.91 §2.4.4 다이렉트 메모리 OOM
// VM 옵션: -Xmx20m -XX:MaxDirectMemorySize=10m
// 기대 결과: java.lang.OutOfMemoryError: Unable to allocate <N> bytes
//
// 주의: sun.misc.Unsafe 는 내부 API. JDK 21에서는 jdk.internal.misc.Unsafe 사용이 권장되지만,
//      책의 코드 그대로 재현하기 위해 sun.misc.Unsafe 를 reflection으로 꺼낸다.
public final class DirectMemoryOOM {

    private static final int _1MB = 1024 * 1024;

    public static void main(String[] args) throws Exception {
        Field unsafeField = Unsafe.class.getDeclaredFields()[0];
        unsafeField.setAccessible(true);
        Unsafe unsafe = (Unsafe) unsafeField.get(null);
        while (true) {
            unsafe.allocateMemory(_1MB);
        }
    }
}
