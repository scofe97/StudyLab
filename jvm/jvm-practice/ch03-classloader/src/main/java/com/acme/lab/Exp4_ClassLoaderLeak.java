package com.acme.lab;

import java.net.URL;
import java.net.URLClassLoader;
import java.util.ArrayList;
import java.util.List;

// 실험 4 — ClassLoader Leak 과 Metaspace OOM
//
// 톰캣 redeploy 는 옛 WebApp 로더를 버리고 새 로더를 만든다. 옛 로더가 GC 되면
// 그 로더가 정의한 클래스 메타데이터(Metaspace)도 회수된다. 그런데 누군가
// 옛 로더를 계속 참조하면 회수되지 못하고 Metaspace 가 차오른다.
//
// 이 실험은 "redeploy" 를 반복하며 매번 새 URLClassLoader 로 클래스를 로딩한다.
//   - LEAK=false: 로더를 참조에서 놓아준다 → GC 가 회수 → 안정
//   - LEAK=true : 로더를 static 리스트와 ThreadLocal 에 붙잡는다 → 회수 불가 → OOM
//
// 실행:
//   java -XX:MaxMetaspaceSize=32m ... Exp4_ClassLoaderLeak leak    # OOM 재현
//   java -XX:MaxMetaspaceSize=32m ... Exp4_ClassLoaderLeak noleak  # 안정
public class Exp4_ClassLoaderLeak {

    // 누수 원인 ①: static 컬렉션이 로더를 영구 참조
    static final List<ClassLoader> LEAKED_LOADERS = new ArrayList<>();

    // 누수 원인 ②: ThreadLocal 에 로더가 로딩한 클래스 인스턴스가 남음
    static final ThreadLocal<Object> LEAKY_TL = new ThreadLocal<>();

    public static void main(String[] args) throws Exception {
        boolean leak = args.length > 0 && args[0].equalsIgnoreCase("leak");
        System.out.println("모드 = " + (leak ? "LEAK (누수 — OOM 예상)" : "NO-LEAK (정상 — 안정)"));
        System.out.println("MaxMetaspaceSize 를 작게 주고 실행하세요 (예: -XX:MaxMetaspaceSize=32m)");
        System.out.println();

        URL pluginJar = LabPaths.dir("plugin");
        URL apiJar    = LabPaths.dir("app");
        ClassLoader app = Exp4_ClassLoaderLeak.class.getClassLoader();

        for (int i = 1; i <= 100_000; i++) {
            // 매 라운드가 한 번의 redeploy 라고 보면 된다: 새 로더 + 새 클래스 정의
            URLClassLoader loader =
                    new URLClassLoader(new URL[]{pluginJar, apiJar}, app);

            Class<?> plugin = loader.loadClass("com.acme.plugin.HelloPlugin");
            Object instance = plugin.getDeclaredConstructor().newInstance();

            if (leak) {
                // 옛 로더를 놓아주지 않는다 → GC 가 회수하지 못함
                LEAKED_LOADERS.add(loader);
                LEAKY_TL.set(instance);   // ThreadLocal 도 인스턴스(→로더)를 붙잡음
            } else {
                // 참조를 끊는다 → loader/plugin/instance 모두 GC 대상
                loader.close();
                instance = null;
            }

            if (i % 200 == 0) {
                long meta = metaspaceUsedMB();
                System.out.printf("round %5d  | leaked loaders=%6d | Metaspace≈%4d MB%n",
                        i, leak ? LEAKED_LOADERS.size() : 0, meta);
            }
        }
        System.out.println("완료(누수 모드라면 여기 도달 전에 OutOfMemoryError: Metaspace).");
    }

    // Metaspace 사용량(MB) 근사 — java.lang.management 로 조회
    static long metaspaceUsedMB() {
        for (java.lang.management.MemoryPoolMXBean pool :
                java.lang.management.ManagementFactory.getMemoryPoolMXBeans()) {
            if (pool.getName().contains("Metaspace")) {
                return pool.getUsage().getUsed() / (1024 * 1024);
            }
        }
        return -1;
    }
}
