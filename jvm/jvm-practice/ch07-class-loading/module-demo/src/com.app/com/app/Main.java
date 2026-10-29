package com.app;

import com.lib.api.PublicApi;            // OK — com.lib 가 api 를 exports 함
// import com.lib.internal.SecretImpl;   // ← 주석 풀면 컴파일 에러! (internal 은 exports 안 됨)

public class Main {
    public static void main(String[] args) {
        // 공개 API 는 정상 사용
        PublicApi api = new PublicApi();
        System.out.println(api.hello());

        // 아래 주석을 풀면 컴파일 에러로 강한 캡슐화를 확인할 수 있다:
        //   SecretImpl secret = new SecretImpl();
        //   System.out.println(secret.secret());
        // 에러: package com.lib.internal is not visible
        //       (package com.lib.internal is declared in module com.lib,
        //        which does not export it)
        System.out.println("internal 은 import 조차 막힌다 (Main.java 주석 참고)");
    }
}
