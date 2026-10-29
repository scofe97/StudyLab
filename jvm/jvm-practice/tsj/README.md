# tsj — Troubleshooting Java 실습 앱

『Troubleshooting Java, 2nd Edition』(Laurențiu Spilcă, Manning) 책의 저자 제공 실습 앱 모음이다. 각 앱은 원본 Maven 구조 그대로 보존한다.

## 환경

| 항목 | 값 |
|------|------|
| 빌드 도구 | Maven (원본 유지) |
| Java | 17 (순수 Java 앱) / Spring Boot 앱은 parent pom 버전 따름 |
| 소스 | Manning 공식 소스코드 다운로드 (`/downloads/2464`) |

## 실행

```bash
# 순수 Java 앱 (ch1~ch3, ch5~ch6, ch9~ch11-ex2)
cd tsj/da-ch1-ex1
mvn compile exec:java -Dexec.mainClass="com.example.Main"

# Spring Boot 앱 (ch4, ch7~ch8, ch11-ex1, ch12)
cd tsj/da-ch4-ex1
mvn spring-boot:run
```

## 앱 ↔ 학습 노트 매핑

| 앱 | 책 장 | 핵심 주제 | 노트 |
|----|-------|----------|------|
| `da-ch1-ex1` | 1장 | 난독 코드 — 디버거로 따라가기 (listing 1.1) | `01-01` |
| `da-ch1-ex2` | 1장 | 네 시나리오 실습 | `01-02` |
| `da-ch2-ex1` | 2장 | 디버거 기초 — 중단점·step over/into | `02-01` |
| `da-ch2-ex2` | 2장 | 스택 트레이스·코드 네비게이션 | `02-02` |
| `da-ch2-ex3` | 2장 | 디버거가 부족한 상황 | `02-03` |
| `da-ch3-ex1` | 3장 | 조건부 중단점·비중단 중단점 | `03-01` |
| `da-ch3-ex2` | 3장 | 인메모리 데이터 변경·프레임 되감기 | `03-02` |
| `da-ch4-ex1` | 4장 | 로그 조사 (Spring Boot + Logback) | `04-01`~`04-03` |
| `da-ch5-ex1` | 5장 | 프로파일러 도입 | `05-01` |
| `da-ch5-ex2` | 5장 | VisualVM CPU·스레드·메모리 누수 | `05-02`~`05-03` |
| `da-ch6-ex1` | 6장 | 샘플링으로 실행 코드 관찰 | `06-01` |
| `da-ch6-ex2` | 6장 | instrumentation·JDBC SQL 가로채기 (N+1) | `06-02` |
| `da-ch6-ex3` | 6장 | Hibernate 생성 SQL·criteria cross join | `06-03` |
| `da-ch7-ex1` | 7장 | 스레드 락 모니터링 (producer-consumer) | `07-01`~`07-02` |
| `da-ch7-ex2` | 7장 | wait·notify 대기 함정 | `07-03` |
| `da-ch7-ex3` | 7장 | JPA 락 시나리오 | `07-*` |
| `da-ch7-ex4` | 7장 | JPA 락 시나리오 (변형) | `07-*` |
| `da-ch8-ex1` | 8장 | 데드락 — 중첩 synchronized | `08-01`~`08-02` |
| `da-ch8-ex2` | 8장 | 스레드 덤프 분석 (변형) | `08-03` |
| `da-ch8-ex3` | 8장 | MongoDB 스레드 덤프 | `08-03` |
| `da-ch9-ex1` | 9장 | 메모리 샘플링·할당 문제 | `09-01`~`09-02` |
| `da-ch9-ex2` | 9장 | 메모리 프로파일링 (변형) | `09-02` |
| `da-ch10-ex1` | 10장 | 힙 덤프·OQL (static 리스트 누수) | `10-01`~`10-03` |
| `da-ch11-ex1` | 11장 | GC 로그 (Spring Boot) | `11-01`~`11-03` |
| `da-ch11-ex2` | 11장 | GC 로그 (순수 Java) | `11-01`~`11-03` |
| `da-ch12-ex1` | 12장 | 분산 추적·직렬화 | `12-01`~`12-02` |
| `da-ch12-ex2` | 12장 | 시스템 장애 모드 | `12-03` |
| `da-app-e-ex1` | 부록 | 부록 예제 | — |

## 노트 위치

`~/Library/CloudStorage/GoogleDrive-.../study/runners-high/write/01_language/java/05_JVM/tsj_troubleshooting-java/`
