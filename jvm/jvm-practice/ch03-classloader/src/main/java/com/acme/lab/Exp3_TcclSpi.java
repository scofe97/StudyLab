package com.acme.lab;

import com.acme.spi.Codec;
import java.net.URL;
import java.net.URLClassLoader;
import java.util.ServiceLoader;

// 실험 3 — 부모 위임의 한계와 TCCL(Thread Context ClassLoader)
//
// 부모(앱) 로더는 구조상 자식 로더가 로딩한 구현체를 직접 못 본다.
// ServiceLoader.load(Codec.class) 를 부모 로더로 돌리면 구현체를 못 찾는다.
//   - 케이스 A: ServiceLoader.load(Codec.class) → 앱 로더 기준 → 0개
//   - 케이스 B: ServiceLoader.load(Codec.class, 자식로더) → 1개 발견
//   - 케이스 C: TCCL 을 자식 로더로 바꾸고 ServiceLoader.load(Codec.class) → 1개
//
// 톰캣이 요청 스레드의 TCCL 을 WebApp 로더로 맞춰 두면, 부모 계층의 SPI 코드가
// 현재 웹앱 구현체를 찾을 수 있게 되는 것과 같은 원리다.
public class Exp3_TcclSpi {

    public static void main(String[] args) throws Exception {
        URL providerJar = LabPaths.dir("provider");
        ClassLoader app = Exp3_TcclSpi.class.getClassLoader();

        // 자식 로더: provider 구현 + META-INF/services 를 들고 있다.
        // Codec 인터페이스(com.acme.spi)는 부모와 공유해야 같은 타입이므로 parent-first.
        ChildFirstClassLoader child =
                new ChildFirstClassLoader(new URL[]{providerJar}, app, "com.acme.spi");

        System.out.println("== 케이스 A: 부모(앱) 로더 기준 ServiceLoader ==");
        int countA = 0;
        for (Codec c : ServiceLoader.load(Codec.class)) {
            countA++;
            System.out.println("  발견: " + c);
        }
        System.out.println("  구현체 수 = " + countA + "  (부모는 자식 구현을 못 본다)");

        System.out.println();

        System.out.println("== 케이스 B: 자식 로더를 명시한 ServiceLoader ==");
        int countB = 0;
        for (Codec c : ServiceLoader.load(Codec.class, child)) {
            countB++;
            System.out.println("  발견: " + c + " → " + c.encode("tomcat"));
        }
        System.out.println("  구현체 수 = " + countB);

        System.out.println();

        System.out.println("== 케이스 C: TCCL 을 자식으로 바꾼 뒤 ServiceLoader ==");
        ClassLoader saved = Thread.currentThread().getContextClassLoader();
        try {
            Thread.currentThread().setContextClassLoader(child);
            int countC = 0;
            // load(Codec.class) 는 내부적으로 현재 스레드의 TCCL 을 쓴다.
            for (Codec c : ServiceLoader.load(Codec.class)) {
                countC++;
                System.out.println("  발견: " + c + " → " + c.encode("tomcat"));
            }
            System.out.println("  구현체 수 = " + countC + "  (TCCL 이 통로가 됐다)");
        } finally {
            Thread.currentThread().setContextClassLoader(saved); // 반드시 원복(실험 4 의 누수 원인)
        }
    }
}
