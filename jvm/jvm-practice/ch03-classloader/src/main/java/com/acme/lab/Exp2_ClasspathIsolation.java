package com.acme.lab;

import java.lang.reflect.Method;
import java.net.URL;
import java.net.URLClassLoader;
import java.security.CodeSource;

// 실험 2 — 클래스패스 격리와 버전 충돌(NoSuchMethodError)
//
// 같은 FQCN(com.acme.Greeter)을 v1/v2 두 디렉터리로 따로 컴파일해 두고,
//   - 로더 A: out/v1 만 클래스패스에 → Greeter.version()="v1", hi() 없음
//   - 로더 B: out/v2 만 클래스패스에 → Greeter.version()="v2", hi() 있음
// 두 로더가 같은 이름의 클래스를 서로 다른 버전으로 본다(= 격리).
//
// 그리고 v1 로더에서 hi() 를 리플렉션으로 부르면 NoSuchMethodError 가 난다.
// "클래스는 있는데 그 메서드만 없다" = 버전 충돌의 전형적 신호.
public class Exp2_ClasspathIsolation {

    public static void main(String[] args) throws Exception {

        URL v1 = LabPaths.dir("v1");
        URL v2 = LabPaths.dir("v2");

        URLClassLoader loaderA = new URLClassLoader(new URL[]{v1}, null); // parent=null: 부트스트랩만
        URLClassLoader loaderB = new URLClassLoader(new URL[]{v2}, null);

        Class<?> gA = loaderA.loadClass("com.acme.Greeter");
        Class<?> gB = loaderB.loadClass("com.acme.Greeter");

        System.out.println("== 격리: 같은 이름, 다른 버전 ==");
        System.out.println("gA = " + gA + "  loader=" + gA.getClassLoader());
        System.out.println("gB = " + gB + "  loader=" + gB.getClassLoader());
        System.out.println("gA == gB ? " + (gA == gB) + "  (이름 같아도 다른 타입)");
        System.out.println("gA 출처 = " + codeSource(gA));
        System.out.println("gB 출처 = " + codeSource(gB));

        Object a = gA.getDeclaredConstructor().newInstance();
        Object b = gB.getDeclaredConstructor().newInstance();
        System.out.println("A.version() = " + gA.getMethod("version").invoke(a));
        System.out.println("B.version() = " + gB.getMethod("version").invoke(b));

        System.out.println();
        System.out.println("== 버전 충돌: v1 로더에서 hi() 호출 ==");
        try {
            // v1 에는 hi() 가 없다. getMethod 단계에서 NoSuchMethodException,
            // 컴파일된 코드가 직접 부르는 상황을 흉내 내려 reflect 로 탐색한다.
            Method hi = gA.getMethod("hi");
            System.out.println("hi() 찾음?! → " + hi.invoke(a));
        } catch (NoSuchMethodException e) {
            System.out.println("NoSuchMethodException: " + e.getMessage());
            System.out.println("→ v1 Greeter 에는 hi() 가 없다. v2 라고 착각하고 부르면 깨진다.");
        }

        // 대조: v2 로더에서는 hi() 가 정상 동작
        Method hiB = gB.getMethod("hi");
        System.out.println("B.hi() = " + hiB.invoke(b) + "  (v2 에는 있음)");
    }

    static String codeSource(Class<?> c) {
        CodeSource cs = c.getProtectionDomain().getCodeSource();
        return cs == null ? "(부트스트랩/알 수 없음)" : cs.getLocation().toString();
    }
}
