package com.acme.lab;

import java.net.URL;
import java.net.URLClassLoader;

// 톰캣 WebApp 로더의 핵심 성질을 흉내 낸 child-first(자식 우선) 로더.
//
// 표준 부모 위임(URLClassLoader 기본)은 부모에게 먼저 묻는다.
// WebApp 로더는 웹앱 독립성을 위해 *자기 클래스패스를 먼저* 뒤지고,
// 없을 때만 부모로 위임한다. 단 java.* / javax.* 같은 핵심 클래스와
// (여기서는) 공용 계약 패키지는 부모에 먼저 위임해 타입 호환을 지킨다.
public class ChildFirstClassLoader extends URLClassLoader {

    private final String[] parentFirstPrefixes;

    public ChildFirstClassLoader(
            URL[] urls
            , ClassLoader parent
            , String... parentFirstPrefixes
    ) {
        super(urls, parent);
        this.parentFirstPrefixes = parentFirstPrefixes;
    }

    @Override
    protected Class<?> loadClass(String name, boolean resolve) throws ClassNotFoundException {
        // JDK 제공함수 (다른 스레드에서 동일 이름 접근시 이름 클래스 . 제공)
        synchronized (getClassLoadingLock(name)) {
            // 1) 이미 이 로더가 정의/로딩한 클래스면 그대로 반환
            Class<?> c = findLoadedClass(name);
            if (c != null) {
                return resolved(c, resolve);
            }

            // 2) 핵심 클래스와 부모-우선 지정 패키지는 부모에 먼저 위임
            //    (java.* 를 자식이 덮어쓰면 안 되고, 공용 계약은 한 벌만 공유돼야 함)
            if (isParentFirst(name)) {
                return resolved(super.loadClass(name, resolve), resolve);
            }

            // 3) 그 외에는 자기 클래스패스를 먼저 뒤진다 — child-first
            try {
                c = findClass(name);
                return resolved(c, resolve);
            } catch (ClassNotFoundException notHere) {
                // 4) 자기 클래스패스에 없으면 그때 부모로 위임
                return resolved(super.loadClass(name, resolve), resolve);
            }
        }
    }

    private boolean isParentFirst(String name) {
        if (name.startsWith("java.") || name.startsWith("javax.") || name.startsWith("jdk.")) {
            return true;
        }
        for (String prefix : parentFirstPrefixes) {
            if (name.startsWith(prefix)) {
                return true;
            }
        }
        return false;
    }

    private Class<?> resolved(Class<?> c, boolean resolve) {
        if (resolve) {
            resolveClass(c);
        }
        return c;
    }

    @Override
    public String toString() {
        return "ChildFirstClassLoader@" + Integer.toHexString(hashCode());
    }
}
