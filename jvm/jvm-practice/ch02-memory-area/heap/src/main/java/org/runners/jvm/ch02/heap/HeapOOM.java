package org.runners.jvm.ch02.heap;

import java.util.ArrayList;
import java.util.List;

// 책 p.77 §2.4.1 자바 힙 오버플로
// VM 옵션: -Xms20m -Xmx20m -XX:+HeapDumpOnOutOfMemoryError (build.gradle.kts에서 박제)
// 기대 결과: java.lang.OutOfMemoryError: Java heap space
public final class HeapOOM {

    static class OOMObject {}

    public static void main(String[] args) {
        List<OOMObject> list = new ArrayList<>();
        while (true) {
            list.add(new OOMObject());
        }
    }
}
