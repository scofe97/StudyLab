// 라이브러리 모듈 — api 패키지만 공개(exports), internal 은 숨김
module com.lib {
    exports com.lib.api;        // 공개 — 외부 모듈이 쓸 수 있다
    // com.lib.internal 은 exports 하지 않음 → 모듈 밖에서 접근 불가
}
