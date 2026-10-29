package com.lib.api;

// 공개 API — com.lib 모듈이 exports 하는 패키지에 있음 → 외부 모듈이 쓸 수 있다
public class PublicApi {
    public String hello() {
        return "PublicApi.hello() — 공개된 api 패키지";
    }
}
