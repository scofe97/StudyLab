package com.lib.internal;

// 내부 구현 — com.lib 모듈이 exports 하지 *않는* 패키지에 있음
// public 이지만 모듈 밖에서는 접근 불가 (강한 캡슐화)
public class SecretImpl {
    public String secret() {
        return "SecretImpl.secret() — 숨겨진 internal 패키지";
    }
}
