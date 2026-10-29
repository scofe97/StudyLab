package com.myapp.payment;

import com.mylib.spi.PaymentProvider;

// 구현체 2 — 새 결제수단을 "코드 수정 없이" 추가하는 것을 보여주려는 두 번째 제공자.
// 이걸 등록 파일에 한 줄 추가하기만 하면 ServiceLoader 가 함께 찾아낸다.
public class KakaoPayment implements PaymentProvider {

    @Override
    public boolean supports(String type) {
        return "kakao".equals(type);
    }

    @Override
    public String pay(long amount) {
        return "[Kakao] " + amount + "원 결제 완료";
    }
}
