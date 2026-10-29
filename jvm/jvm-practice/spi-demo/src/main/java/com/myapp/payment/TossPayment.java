package com.myapp.payment;

import com.mylib.spi.PaymentProvider;

// 구현체 1 — "사용자(또는 내가)" 작성하는 결제 제공자.
// 이 클래스 이름은 라이브러리 코드 어디에도 import 되지 않는다.
// META-INF/services 등록 파일이 유일한 연결고리다.
public class TossPayment implements PaymentProvider {

    @Override
    public boolean supports(String type) {
        return "toss".equals(type);
    }

    @Override
    public String pay(long amount) {
        return "[Toss] " + amount + "원 결제 완료";
    }
}
