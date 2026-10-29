package com.acme;

// 라이브러리 v1 — hello() 만 있다.
// experiment 2 에서 v2 와 같은 FQCN(com.acme.Greeter)으로 컴파일해
// "같은 이름, 다른 버전"을 두 로더에 각각 올린다.
public class Greeter {
    public String version() {
        return "v1";
    }

    public String hello() {
        return "hello from " + version();
    }
}
