package com.acme.spi;

// SPI 계약. 부모(앱) 로더에 놓인다. JDBC 의 java.sql.Driver,
// 톰캣의 Common 계층 API 와 같은 자리 — "컨테이너/플랫폼이 들고 있는 계약".
public interface Codec {
    String encode(String s);
}
