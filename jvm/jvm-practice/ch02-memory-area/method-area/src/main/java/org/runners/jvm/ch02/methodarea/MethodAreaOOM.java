package org.runners.jvm.ch02.methodarea;

import net.bytebuddy.ByteBuddy;
import net.bytebuddy.dynamic.loading.ClassLoadingStrategy;

// 책 p.89 §2.4.3 메서드 영역 OOM
// JDK 21 변경 사유: 책의 CGLib Enhancer 예제는 JDK 17+에서 reflection 차단으로 동작이 불안정.
//   ByteBuddy로 동일한 효과(매번 새 클래스 생성)를 재현한다.
// VM 옵션: -XX:MetaspaceSize=10m -XX:MaxMetaspaceSize=10m
// 기대 결과: java.lang.OutOfMemoryError: Metaspace
public final class MethodAreaOOM {

    static class OOMObject {}

    public static void main(String[] args) {
        long counter = 0;
        while (true) {
            new ByteBuddy()
                .subclass(OOMObject.class)
                .name("org.runners.jvm.ch02.methodarea.Generated$" + counter++)
                .make()
                .load(MethodAreaOOM.class.getClassLoader(), ClassLoadingStrategy.Default.WRAPPER);
        }
    }
}
