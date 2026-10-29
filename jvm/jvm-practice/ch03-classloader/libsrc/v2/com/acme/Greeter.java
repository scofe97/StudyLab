package com.acme;

// 라이브러리 v2 — hello() 에 더해 hi() 가 추가됐다.
// v1 에는 hi() 가 없으므로, v1 이 먼저 로딩된 로더에서 hi() 를 호출하면
// NoSuchMethodError 가 난다(클래스는 있는데 그 메서드만 없는 상황).
public class Greeter {
    public String version() {
        return "v2";
    }

    public String hello() {
        return "hello from " + version();
    }

    public String hi() {
        return "hi from " + version();
    }
}
