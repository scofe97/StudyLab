package com.acme.api;

// 공용 계약(interface). 톰캣의 Common 계층에 놓인 servlet-api 처럼,
// "여러 로더가 공유해야 타입을 주고받을 수 있는" 클래스의 역할이다.
// 이 인터페이스를 어느 로더가 로딩하느냐가 experiment 1 의 핵심이다.
public interface Plugin {
    String run();
}
