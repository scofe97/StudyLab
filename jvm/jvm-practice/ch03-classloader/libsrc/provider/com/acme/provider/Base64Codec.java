package com.acme.provider;

import com.acme.spi.Codec;
import java.util.Base64;

// SPI 구현체. 웹앱(자식 로더) 안에 놓이는 구현으로, 부모 계층 코드인
// ServiceLoader 가 *자식 쪽 구현체*를 찾아야 하는 상황을 만든다.
public class Base64Codec implements Codec {
    @Override
    public String encode(String s) {
        return Base64.getEncoder().encodeToString(s.getBytes());
    }
}
