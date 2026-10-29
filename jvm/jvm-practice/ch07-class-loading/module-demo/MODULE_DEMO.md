# 모듈 실습 — exports 한 패키지 vs 안 한 패키지 (강한 캡슐화)

> ch03 02-05 노트(자바 모듈 시스템)의 Phase 3 실습. module-info.java 로 두 모듈을 만들어
> exports 한 패키지는 쓰이고, exports 안 한 패키지는 public 이어도 import 조차 막히는 것을 본다.

## 관련 이론
- [02-05. 자바 모듈 시스템과 클래스 로더 변화](../../../ch03_class-loading-mechanism/02-05.자바 모듈 시스템과 클래스 로더 변화.md) §1

## 실습 구조
```
src/
  com.lib/                     라이브러리 모듈
    module-info.java           exports com.lib.api;  (internal 은 숨김)
    com/lib/api/PublicApi.java     공개
    com/lib/internal/SecretImpl.java  숨김(public 이지만 exports 안 됨)
  com.app/                     앱 모듈
    module-info.java           requires com.lib;
    com/app/Main.java
```
- JDK: Temurin 21.0.3.

## ① 정상 빌드·실행 — api 패키지는 쓰인다
```
$ javac -d out --module-source-path src $(find src -name "*.java")
$ java --module-path out -m com.app/com.app.Main
PublicApi.hello() — 공개된 api 패키지
```
`com.lib` 가 `exports com.lib.api` 하므로, `requires com.lib` 한 `com.app` 이 `PublicApi` 를 쓴다.

## ② 강한 캡슐화 — internal 은 public 이어도 안 보인다 (핵심)

`Main.java` 에서 `import com.lib.internal.SecretImpl;` 주석을 풀고 컴파일하면:
```
error: package com.lib.internal is not visible
import com.lib.internal.SecretImpl;
              ^
  (package com.lib.internal is declared in module com.lib, which does not export it)
```
- `SecretImpl` 은 `public` 인데도 import 조차 안 된다. `com.lib` 가 `internal` 을 exports 하지
  않았기 때문이다.
- 클래스패스 시대에는 public 이면 누구나 접근 가능했지만, 모듈 시대에는 *public + exports* 라야
  공개다. 이것이 패키지 레벨의 강한 캡슐화.

## ③ 열어주면 통과 — exports 추가
`com.lib/module-info.java` 에 `exports com.lib.internal;` 을 추가하면 ②의 에러가 사라진다.
exports 선언 유무가 접근을 가르는 것을 양방향으로 확인.

## classpath vs module-path

같은 코드라도 어떻게 띄우냐로 캡슐화가 갈린다.
```
java -cp out com.app.Main            ← 클래스패스 모드: 캡슐화 무시(unnamed module)
java --module-path out -m com.app/com.app.Main  ← 모듈 경로 모드: exports/requires 강제
```
- 클래스패스(`-cp`)는 모든 JAR 가 한 평면에 펼쳐져 경계가 없다. module-info 가 있어도 클래스패스에
  두면 unnamed module 로 취급돼 캡슐화가 적용되지 않는다.
- 모듈 경로(`--module-path`)로 띄워야 비로소 exports 안 한 패키지가 차단된다.
- 대부분의 Spring 앱이 여전히 클래스패스 모드로 도는 이유 — 모듈 경로의 강제 캡슐화가 리플렉션
  범벅인 Spring 과 잘 안 맞기 때문(노트 §3).

## 배운 점 (이론 ↔ 실습 연결)

- **public ≠ 공개**: 모듈 시대엔 exports 해야 공개. internal 의 public 클래스가 import 조차
  막히는 걸 컴파일 에러로 실증.
- **클래스패스 vs 모듈 경로**: 같은 .class 도 -cp 로 띄우면 캡슐화 0, --module-path 로 띄우면 강제.
- **requires 의 역할**: com.app 이 com.lib 를 requires 선언해야 그 exports 패키지를 쓸 수 있다.

## 비고
- `out/`·`*.class` 는 컴파일 산출물이라 Drive 에 올리지 않는다(소스·기록만).
- 재현: `cd ~/jvm-practice/ch07-class-loading/module-demo && javac -d out --module-source-path src $(find src -name "*.java") && java --module-path out -m com.app/com.app.Main`. ② 는 Main.java 주석 풀고 재컴파일.
