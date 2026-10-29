# spi-demo — META-INF/services 직접 작성 실습

순수 자바 SPI(`ServiceLoader` + `META-INF/services`)를 *손으로* 작성해 보는 실습이다.
"구현체 이름을 코드가 모르는데도, 등록 파일만으로 런타임에 끼워지는" SPI의 본질을 직접 확인한다.
(ch03-classloader 실험 3 의 SPI 개념을 실무 형태로 옮긴 것이다.)

## 구성

```text
spi-demo/
├── src/main/java/
│   ├── com/mylib/spi/PaymentProvider.java   ← 계약(인터페이스) — 라이브러리가 제공
│   ├── com/myapp/payment/TossPayment.java   ← 구현체 1
│   ├── com/myapp/payment/KakaoPayment.java  ← 구현체 2
│   └── com/myapp/SpiDemo.java               ← 라이브러리 코드(구현체를 import 안 함!)
└── src/main/resources/
    └── META-INF/services/
        └── com.mylib.spi.PaymentProvider    ← ★ 손으로 작성하는 등록 파일
```

핵심: `SpiDemo.java` 어디에도 `TossPayment`/`KakaoPayment` 가 `import` 되지 않는다.
`ServiceLoader` 가 등록 파일을 읽어 구현체를 찾아 준다.

## 실행

```bash
cd ~/jvm-practice
./gradlew :spi-demo:run --args="toss 10000"    # [Toss] 10000원 결제 완료
./gradlew :spi-demo:run --args="kakao 5000"     # [Kakao] 5000원 결제 완료
./gradlew :spi-demo:run --args="naver 1000"     # 지원 제공자 없음
./gradlew :spi-demo:run --args="manual"          # ★ ServiceLoader 없이 META-INF 직접 읽기
```

## manual 모드 — META-INF 가 "어떻게" 읽히는지 학습

`--args="manual"` 은 `ServiceLoader` 를 쓰지 않고, `getResources()` 로 등록 파일을
*직접* 읽어 구현체를 만든다. `ServiceLoader.load()` 가 내부에서 하는 일을 손으로 재현한 것이다.

```java
// ServiceLoader 의 핵심 — 클래스 로더에게 등록 "파일" 을 요청
Enumeration<URL> urls = cl.getResources("META-INF/services/com.mylib.spi.PaymentProvider");
// 파일을 한 줄씩 읽어 구현체 FQCN 을 얻고 → Class.forName 으로 로딩 → 인스턴스 생성
```

출력에서 등록 파일의 *실제 위치*(`build/resources/main/META-INF/...`)와, 파일을 한 줄씩
읽어 구현체를 만드는 과정이 그대로 보인다. 결과는 `ServiceLoader` 방식과 같다.

> 이걸로 알게 되는 것: **`ServiceLoader` 는 마법이 아니라 `getResources()` + 줄 읽기 + `Class.forName` 의 래퍼다.** META-INF/services 는 클래스 로더가 특별 취급하는 폴더가 아니라, `getResources()` 로 *요청하면 주는* 평범한 리소스 파일일 뿐이다. (ch03-classloader 실험 3 §ServiceLoader 5단계 흐름을 코드로 직접 짠 것.)

## 직접 해볼 실습

**① 등록 파일을 고치면 동작이 바뀐다.**
`src/main/resources/META-INF/services/com.mylib.spi.PaymentProvider` 에서
`com.myapp.payment.KakaoPayment` 줄 앞에 `#` 를 붙여 주석 처리하고 다시 실행해 보라.

```bash
./gradlew :spi-demo:run --args="kakao 5000"
```

→ 등록 목록이 2개에서 1개로 줄고, `kakao` 결제가 "지원 제공자 없음" 이 된다.
**코드는 한 줄도 안 고쳤는데** 등록 파일만으로 동작이 바뀐다. 이게 SPI 다.

**② 새 결제수단을 코드 수정 없이 추가한다.**
`NaverPayment.java` 를 `TossPayment` 처럼 작성하고, 등록 파일에 한 줄
`com.myapp.payment.NaverPayment` 만 추가하면 `--args="naver ..."` 가 동작한다.
`SpiDemo.java` 는 건드리지 않는다.

## 실무에서 어디서 보나

- **JDBC 드라이버**: MySQL JAR 안에 `META-INF/services/java.sql.Driver` 가 있다.
  `DriverManager` 가 이걸 읽어 드라이버를 찾는다. 매일 쓰는 SPI.
- **SLF4J**: `META-INF/services/org.slf4j.spi.SLF4JServiceProvider`
- **라이브러리 확장점**: 내가 라이브러리를 만들어 "사용자가 구현을 끼울 수 있게" 할 때.
- **`@AutoService`**(Google): 손으로 등록 파일 쓰는 대신 애너테이션으로 컴파일 때 자동 생성.

> Spring 앱 안에서는 `@Component`/`@Bean`(DI)이 이 역할을 대신해 직접 쓸 일이 드물다.
> 프레임워크/라이브러리를 *만드는* 쪽이거나 JDBC 같은 표준 SPI 를 다룰 때 마주친다.
