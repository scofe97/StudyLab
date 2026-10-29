# 04-01 톰캣 클래스 로더 — 직접 재현 실습

학습 노트 `write/01_language/java/05_JVM/ch03_class-loading-mechanism/04-01.톰캣의 클래스 로더 아키텍처.md`의
핵심 현상 4가지를, **톰캣 없이 순수 Java + `URLClassLoader`** 로 눈으로 확인하는 실습이다.
톰캣의 WebApp 로더가 하는 일을 `ChildFirstClassLoader` 한 클래스로 흉내 내, 노트 개념과 1:1로 맞췄다.

- 실행 JDK: **Java 21 toolchain** (루트 `build.gradle.kts` 가 지정 — 기본 PATH 의 Java 25 와 무관)
- 이 폴더는 `jvm-practice`(`jvm-deep-dive`) Gradle 프로젝트의 `:ch03-classloader` 모듈이다.

### IntelliJ

`jvm-practice` 를 IntelliJ 로 열면 `:ch03-classloader` 가 모듈로 잡힌다.
Gradle 패널의 `Tasks > lab` 아래 `exp1`~`exp4NoLeak` 을 더블클릭하면 실행된다.
(각 실험은 `main` 이 따로라, IntelliJ 의 일반 "Run" 보다 이 Gradle 태스크로 실행하는 게 정확하다.)

### 명령행 (Gradle)

```bash
cd ~/jvm-practice
./gradlew :ch03-classloader:exp1        # 실험 1
./gradlew :ch03-classloader:exp2        # 실험 2
./gradlew :ch03-classloader:exp3        # 실험 3
./gradlew :ch03-classloader:exp4Leak    # 실험 4 (OOM 재현 — OOM 나도 BUILD SUCCESS)
./gradlew :ch03-classloader:exp4NoLeak  # 실험 4 (안정 대조)
```

> `scripts/run.sh` 는 Gradle 편입 *이전* 의 단독 실행 스크립트로, 현재 디렉터리 구조와
> 맞지 않는다(`src/` → `src/main/java/`, `out/` → `build/labout/` 로 바뀜). Gradle 태스크를 쓴다.

### lab 산출물 구조

실험들이 `URLClassLoader` 로 따로 로딩하는 클래스는 앱 클래스패스에 두면 안 되므로,
`libsrc/` 에서 `build/labout/<name>` 으로 *분리 컴파일*된다. 위치는 실행 시 `-Dlab.out` 으로 주입한다
(`src/main/java/com/acme/lab/LabPaths.java`).

| labout 디렉터리 | 내용 | 역할 |
|-----------------|------|------|
| `app/` | `Plugin`, `Codec` 인터페이스 (main 출력) | 공유 계약 |
| `v1/`, `v2/` | 같은 `com.acme.Greeter` 두 버전 | 격리·버전 충돌 |
| `plugin/` | `HelloPlugin` | 실험 1·4가 로딩 |
| `provider/` | `Base64Codec` + `META-INF/services` | 실험 3 SPI |

---

## 왜 톰캣 없이 URLClassLoader 인가

톰캣의 WebApp 로더는 결국 "자기 클래스패스를 먼저 뒤지는 자식 우선 `URLClassLoader`"다.
실습에서는 `src/com/acme/lab/ChildFirstClassLoader.java` 가 그 성질(자식 우선 + `java.*`·공용 계약은 부모 우선)을
재현한다. 웹 컨테이너 설정 없이 노트의 §3·§4 메커니즘을 그대로 관찰할 수 있다.

| 톰캣 | 이 실습 |
|------|---------|
| WebApp 로더 (`webapps/appA`) | `ChildFirstClassLoader` 인스턴스 하나 |
| Common 계층 공유 API (servlet-api) | 부모(앱 로더)가 로딩하는 `com.acme.api.Plugin`, `com.acme.spi.Codec` |
| `WEB-INF/lib` 의 웹앱 전용 JAR | `out/v1`, `out/v2`, `out/plugin`, `out/provider` 디렉터리 |
| redeploy = WebApp 로더 통째 교체 | 실험 4 의 라운드마다 새 `URLClassLoader` 생성 |

---

## 실험 1 — 클래스 동일성 = 이름 + 로더 (§4)

**노트 대응**: §4 "격리의 핵심 — 로더가 다르면 타입이 다르다", `ClassCastException: A cannot be cast to A`.

같은 `HelloPlugin.class` 를 두 자식 로더로 각각 로딩한다. FQCN 은 같아도 정의한 로더가 다르니 다른 타입이다.

- **케이스 A** — 공용 계약 `Plugin` 을 부모(앱 로더)가 공유(`parentFirstPrefixes="com.acme.api"`).
  두 자식의 `HelloPlugin` 은 서로 다른 클래스지만, *부모가 로딩한 한 개의 `Plugin`* 으로는 모두 캐스팅된다.
- **케이스 B** — 계약까지 자식이 따로 로딩하면, 부모 `Plugin` 과 자식 `Plugin` 이 다른 타입이 되어 캐스팅이 깨진다.

실측 출력:

```
class com.acme.plugin.HelloPlugin cannot be cast to class com.acme.api.Plugin
(com.acme.plugin.HelloPlugin is in unnamed module of loader com.acme.lab.ChildFirstClassLoader @483bf400;
 com.acme.api.Plugin is in unnamed module of loader 'app')
```

**배우는 점**: 공유돼야 할 API(servlet-api 등)를 웹앱이 자기 `WEB-INF/lib` 에 또 넣으면
바로 이 상황이 된다 — 노트 §3 "컨테이너 제공 API는 웹앱 클래스패스에 넣지 않는다" 의 실패 사례.

## 실험 2 — 클래스패스 격리와 버전 충돌 (§3·§4)

**노트 대응**: §3 "그래서 클래스패스란 무엇인가", §4 오류 표의 `NoSuchMethodError`.

같은 `com.acme.Greeter` 를 v1(메서드 `hello`)·v2(`hello`+`hi`) 두 디렉터리로 따로 컴파일하고,
로더 A는 `out/v1` 만, 로더 B는 `out/v2` 만 클래스패스에 둔다.

- 두 `Greeter` 는 이름이 같아도 다른 타입이고 `getCodeSource()` 출처가 다르다(격리).
- v1 로더에서 `hi()` 를 부르면 없다 → "클래스는 있는데 그 메서드만 없는" 버전 충돌의 전형.

**배우는 점**: `NoSuchMethodError` 가 나면 "라이브러리가 없다"가 아니라
"*의도와 다른 버전*이 그 로더의 클래스패스에서 먼저 잡혔다"를 의심해야 한다.

## 실험 3 — 부모 위임의 한계와 TCCL (§6)

**노트 대응**: §6 "역방향 조회 — TCCL이 필요한 이유".

부모(앱) 로더는 자식 로더가 들고 있는 SPI 구현체를 직접 못 본다. `ServiceLoader` 로 확인한다.

- 케이스 A: 부모 기준 `ServiceLoader.load(Codec.class)` → **0개**
- 케이스 B: 자식 로더를 명시 → **1개**
- 케이스 C: **TCCL** 을 자식으로 바꾸면 `load(Codec.class)` 가 자식 구현을 찾는다 → **1개**

**배우는 점**: 톰캣이 요청 스레드의 TCCL 을 WebApp 로더로 맞춰 두는 이유가 이것이다.
JDBC `DriverManager`, JNDI, 각종 SPI 가 부모 계층 코드이면서 웹앱 구현체를 찾을 수 있는 통로가 TCCL.
(케이스 C 가 `finally` 에서 TCCL 을 원복하는데, 원복을 빼먹으면 그게 실험 4 의 누수 원인이 된다.)

## 실험 4 — ClassLoader Leak → Metaspace OOM (§2·§4)

**노트 대응**: §4 "핫 디플로이의 그림자 — ClassLoader Leak", §7 면접 질문 8.

redeploy 를 흉내 내 매 라운드 새 `URLClassLoader` 로 클래스를 로딩한다.

- `leak` 모드: 옛 로더를 `static` 리스트 + `ThreadLocal` 에 붙잡아 GC 를 막는다.
- `noleak` 모드: `loader.close()` 로 참조를 끊는다.

`-XX:MaxMetaspaceSize=32m` 아래 실측 대조:

| 모드 | Metaspace 추이 | 결과 |
|------|----------------|------|
| `leak` | 라운드마다 증가 (11→15 MB, ~4000 라운드) | `OutOfMemoryError: Metaspace` |
| `noleak` | 13~15 MB 에서 평평 (12,000+ 라운드) | 안정, OOM 없음 |

**배우는 점**: 같은 양의 클래스 로딩인데 결과가 정반대다. 차이는 *옛 로더를 붙잡느냐* 뿐이다.
그래서 운영의 질문은 "reload 가 되는가"가 아니라 "*옛 ClassLoader 가 해제되는가*"다.
누수 원인 4종(스레드·`ThreadLocal`·드라이버·static)이 모두 "옛 로더를 가리키는 참조"라는 한 가지로 모인다.

---

## 더 해볼 것 (선택)

- 실험 4 를 `-Xlog:class+load=info` 와 함께 돌려, 라운드마다 같은 클래스가 *다른 로더로* 다시 로딩되는 로그 확인.
- 실험 4 leak 모드에서 힙 덤프(`-XX:+HeapDumpOnOutOfMemoryError`)를 떠 MAT 로 "왜 옛 로더가 GC Root 에 붙어 있나" 추적.
- 실험 1 케이스 B 를 `delegate=true`(부모 우선) 로 바꿔, 캐스팅이 다시 성공하는지 확인.
