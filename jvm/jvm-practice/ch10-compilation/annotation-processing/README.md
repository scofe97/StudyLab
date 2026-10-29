# 애너테이션 처리기 (JSR-269) — 명명 규약 검사기

플러그인 애너테이션 처리(JSR-269)를 컴파일 파이프라인에 직접 꽂아 보는 실습입니다.
`NameCheckProcessor` 가 `ElementScanner`(visitor)로 구문 트리를 순회하며 자바 명명 규약
위반을 컴파일 시점 경고로 냅니다. 컴파일을 막지 않고 경고만 냅니다.

> ⚠️ 이 디렉토리는 `javac` 직접 실행용입니다 — Gradle 모듈로 등록하지 않았습니다
> (`settings.gradle.kts` 에 `include` 없음). 처리기 등록을 `-processor` 플래그로
> 손수 보는 것이 학습 포인트라, 빌드 스크립트로 감싸지 않았습니다.

## 관련 이론

- write 노트: `05_JVM/ch04_compilation-optimization/01-03.실전 — 플러그인 애너테이션 처리기.md`
- 책: 《JVM 밑바닥까지 파헤치기》 §10.4~§10.5 (책 10장 = 이 디렉토리 `ch10`)

> 디렉토리 번호 `ch10` 은 *책 챕터* 기준입니다. write 노트의 `ch04` 는 *4부* 기준이라
> 번호가 다릅니다 — 같은 주제(프론트엔드 컴파일)를 가리킵니다.

## 실행

```bash
# 1) 처리기 먼저 컴파일 (처리기 자체는 평범한 자바 클래스)
javac NameCheckProcessor.java

# 2) 처리기를 적용해 검사 대상 컴파일
javac -processor NameCheckProcessor -cp . badName.java
```

2단계에서 `badName.java` 의 규약 위반 3건(소문자 클래스 `badName`,
비-ALL_CAPS 상수 `maxSize`, 대문자 메서드 `DoWork`)에 경고가 뜹니다.
정상인 `okMethod` 는 경고가 없습니다. 경고만 나고 컴파일은 성공합니다.

## 구조

| 부분 | 역할 |
|------|------|
| `NameCheckProcessor` (`AbstractProcessor` 상속) | 컴파일러 연결점. javac 가 라운드마다 `process` 를 호출 |
| `NameCheckScanner` (`ElementScanner8` 상속) | 실제 검사. 클래스→`visitType`, 메서드→`visitExecutable`, 변수→`visitVariable` |

`AbstractProcessor` 상속은 JDK 동적 프록시의 `InvocationHandler` 구현과 같은 *표준 확장점*
패턴입니다. 다만 처리기는 **컴파일 시점**에 javac 가 부르고, 프록시는 **런타임**에 JVM 이
부른다는 점이 다릅니다.
