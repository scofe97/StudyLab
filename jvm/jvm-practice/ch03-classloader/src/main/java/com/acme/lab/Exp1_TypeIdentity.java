package com.acme.lab;

import com.acme.api.Plugin;
import java.net.URL;
import java.net.URLClassLoader;

// 실험 1 — "클래스 동일성 = 이름 + 로더"
//
// 같은 HelloPlugin.class 를 두 개의 URLClassLoader 로 각각 로딩한다.
// FQCN 은 똑같지만 정의한 로더가 다르므로 JVM 에게는 서로 다른 타입이다.
//
// 이 실험에서 핵심은 *Plugin 인터페이스를 누가 로딩하느냐*다.
//   - 케이스 A: Plugin 을 부모(앱 로더)가 공유 → 캐스팅 성공
//   - 케이스 B: Plugin 까지 각 자식이 따로 로딩 → ClassCastException
public class Exp1_TypeIdentity {

    public static void main(String[] args) throws Exception {
        URL pluginJar = LabPaths.dir("plugin");   // HelloPlugin 위치
        URL apiJar    = LabPaths.dir("app");      // Plugin 인터페이스 위치

        System.out.println("== 케이스 A: 공용 계약(Plugin)을 부모가 공유 ==");
        caseSharedContract(pluginJar);

        System.out.println();
        System.out.println("== 케이스 B: 공용 계약까지 자식이 따로 로딩 ==");
        caseDuplicatedContract(pluginJar, apiJar);
    }

    // 케이스 A — Plugin 인터페이스는 앱(부모) 로더에 한 벌만 있고,
    // 두 자식 로더는 HelloPlugin 만 따로 로딩한다.
    // 부모-우선 prefix 로 "com.acme.api" 를 지정해 계약을 공유시킨다.
    static void caseSharedContract(URL pluginJar) throws Exception {
        ClassLoader app = Exp1_TypeIdentity.class.getClassLoader();

        ChildFirstClassLoader l1 =
                new ChildFirstClassLoader(new URL[]{pluginJar}, app, "com.acme.api");
        ChildFirstClassLoader l2 =
                new ChildFirstClassLoader(new URL[]{pluginJar}, app, "com.acme.api");

        Object p1 = l1.loadClass("com.acme.plugin.HelloPlugin").getDeclaredConstructor().newInstance();
        Object p2 = l2.loadClass("com.acme.plugin.HelloPlugin").getDeclaredConstructor().newInstance();

        System.out.println("p1 type   = " + p1.getClass());
        System.out.println("p1 loader = " + p1.getClass().getClassLoader());
        System.out.println("p2 loader = " + p2.getClass().getClassLoader());
        System.out.println("p1.getClass() == p2.getClass() ? " + (p1.getClass() == p2.getClass()));

        // Plugin 은 부모가 로딩한 '한 타입'이므로 두 인스턴스 모두 캐스팅 성공
        Plugin asPlugin1 = (Plugin) p1;
        Plugin asPlugin2 = (Plugin) p2;
        System.out.println("캐스팅 성공 → " + asPlugin1.run());
        System.out.println("캐스팅 성공 → " + asPlugin2.run());
        System.out.println("Plugin 인터페이스 로더 = " + Plugin.class.getClassLoader()
                + "  (두 자식이 공유)");
    }

    // 케이스 B — Plugin 인터페이스를 부모-우선에서 빼서, 각 자식 로더가
    // out/app 의 Plugin 까지 *자기 클래스패스에서* 따로 로딩하게 한다.
    // 그러면 부모가 로딩한 Plugin 타입과 자식이 로딩한 Plugin 타입이 달라진다.
    static void caseDuplicatedContract(URL pluginJar, URL apiJar) throws Exception {
        ClassLoader app = Exp1_TypeIdentity.class.getClassLoader();

        // parentFirstPrefixes 를 비워 com.acme.api.Plugin 도 child-first 로 로딩
        ChildFirstClassLoader l1 =
                new ChildFirstClassLoader(new URL[]{pluginJar, apiJar}, app /* no shared prefixes */);

        Object p1 = l1.loadClass("com.acme.plugin.HelloPlugin").getDeclaredConstructor().newInstance();

        Class<?> childPlugin = l1.loadClass("com.acme.api.Plugin");
        System.out.println("부모가 본 Plugin 로더 = " + Plugin.class.getClassLoader());
        System.out.println("자식이 본 Plugin 로더 = " + childPlugin.getClassLoader());
        System.out.println("부모 Plugin == 자식 Plugin ? " + (Plugin.class == childPlugin));

        try {
            // p1 의 실제 타입은 '자식 로더의 Plugin' 을 구현한다.
            // 여기서 부모 타입 Plugin 으로 캐스팅하면 이름은 같아도 타입이 달라 실패한다.
            Plugin asParentPlugin = (Plugin) p1;
            System.out.println("캐스팅 성공?! → " + asParentPlugin.run());
        } catch (ClassCastException e) {
            System.out.println("ClassCastException 발생 ↓");
            System.out.println("  " + e.getMessage());
            System.out.println("→ 이름은 com.acme.api.Plugin 으로 같지만 로더가 달라 다른 타입이다.");
        }
    }
}
