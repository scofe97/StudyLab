package com.mylib.spi;

// SPI 계약(확장점). "라이브러리를 만드는 쪽" 이 제공하는 인터페이스다.
// 라이브러리는 이 인터페이스만 알고, 실제 구현체는 모른다.
// 구현체는 사용자가 클래스패스에 추가하고 META-INF/services 로 등록한다.
public interface PaymentProvider {

    // 이 제공자가 처리할 수 있는 결제 타입인가 ("toss", "kakao" 등)
    boolean supports(String type);

    // 결제 실행 (데모라 콘솔 출력만)
    String pay(long amount);
}
