# JVM 실습 코드

《JVM 밑바닥까지 파헤치기》(저우즈밍) 챕터별 실습 모듈을 모은다. 노트는 `05_JVM/chNN_*/` 폴더에, 실습 코드는 이 `_practice/chNN/` 모듈에 있다. 두 자료를 같은 폴더 안에 두는 이유는 챕터 단위로 노트와 코드를 자주 오가기 때문이다. 자세한 배치 규약은 글로벌 하네스 `~/.claude/skills/content/writing/references/second-brain-harness.md` §4.5 를 따른다.

## 환경

| 항목 | 값 |
|------|------|
| JDK | OpenJDK 21 (Temurin 21.0.3 기준) |
| 빌드 도구 | Gradle 9.x (Kotlin DSL) |
| 모듈 추가 정책 | 책 진도에 맞춰 그때그때 추가, 미진행 챕터 사전 생성 금지 |

JDK 21 toolchain 은 `build.gradle.kts` 에서 강제하므로 로컬 `JAVA_HOME` 이 다른 버전이어도 Gradle 이 자동으로 21을 다운로드·사용한다.

## 실행

루트(`_practice/`)에서 챕터 모듈 task를 호출한다.

```bash
./gradlew :ch01:run                                 # 1장 데모 출력
./gradlew :ch02-memory-area:heap:run                # 2장 Heap OOM 재현
./gradlew :ch03-gc:serial:run                       # 3장 Serial GC 워크로드
```

실습마다 등장하는 `java`/`javac` 옵션은 [`JVM-OPTIONS.md`](./JVM-OPTIONS.md) 에 누적 정리한다. 새 옵션을 만나면 거기에 "어디서 처음 썼는지 + 무엇을 위한 것인지" 를 함께 적는다.

빌드 도구가 숨기는 패키지·JAR·클래스패스 규칙은 독립 실습에서 JDK 명령으로 확인합니다.

```bash
./classpath-basics/scripts/run.sh all
```

GC 비교가 목적인 3장 모듈은 별도 래퍼로 돌리면 GC 로그 위치를 임의 지정할 수 있다.

```bash
./tools/run-with-gc-log.sh :ch03-gc:serial:run /tmp/serial-gc.log
```

`javap` 역어셈블은 챕터 빌드 후 클래스 파일을 직접 가리킨다.

```bash
./gradlew :ch01:classes
./tools/decompile.sh ch01/build/classes/java/main/org/runners/jvm/ch01/JavaTechSystemDemo.class
```

힙 덤프는 가동 중인 JVM 프로세스 ID를 인자로 넘긴다.

```bash
./tools/dump-heap.sh <pid> /tmp/heap.hprof          # jcmd <pid> GC.heap_dump 래퍼
```

## 노트 ↔ 코드 매핑

노트 폴더(`../chNN_*/`)는 절 단위 정독, 코드 모듈(`./chNN*/`)은 그 절을 실행 가능하게 박제한다. 두 자료는 한 챕터 단위로 짝을 이룬다.

| 책 챕터 | 노트 폴더 | 코드 모듈 | 빌드 명령 예 |
|--------|----------|----------|------------|
| 1장 자바 기술 체계 | [`../ch01_java-tech/`](../ch01_java-tech/) | [`:ch01`](./ch01/) | `./gradlew :ch01:run` |
| 2장 메모리 영역 | [`../ch02_memory-area/`](../ch02_memory-area/) | [`:ch02-memory-area:*`](./ch02-memory-area/) | `./gradlew :ch02-memory-area:heap:run` |
| 3장 GC | [`../ch03_gc/`](../ch03_gc/) | [`:ch03-gc:*`](./ch03-gc/) | `./gradlew :ch03-gc:serial:run` |

노트 폴더는 `chNN_topic` 형식(언더스코어), 코드 모듈은 `chNN` 또는 `chNN-topic` 형식(하이픈)을 쓴다. 두 표기가 다른 이유는 노트는 사람이 읽는 카테고리 라벨이고, 코드 모듈은 Gradle `:project` 좌표라서 하이픈이 더 자연스럽기 때문이다.

## 챕터 모듈

각 챕터 실습이 만들어지면 이 표에 추가한다. 상태는 ◻ 미진행, ◐ 일부, ◉ 완료.

| 챕터 | 모듈 | 다루는 책의 절 | 상태 |
|------|------|---------------|------|
| 1장 | `:ch01` | §1.2 데모 + §1.6 JDK 빌드 스크립트 박제 | ◐ |
| 2장 | `:ch02-memory-area:*` | §2.3~§2.4 메모리 영역과 OOM 7종 | ◉ |
| 3장 | `:ch03-gc:*` | §3.1~§3.9 GC 알고리즘과 컬렉터 | ◉ |

미진행 챕터는 행을 추가하지 않는다.

## 패키지·코드 컨벤션

| 항목 | 규칙 |
|------|------|
| 패키지 | `org.runners.jvm.ch{NN}` |
| 출처 코멘트 | 책 예제 그대로 옮긴 클래스 상단에 `// 책 p.{쪽수}` 한 줄 |
| 실행 결과 | 노트 본문에 ```결과 코드블록으로 박제 |

## 의존성

공통 의존성은 루트 `build.gradle.kts` 의 `subprojects` 블록에서 관리한다.

| 라이브러리 | 용도 |
|------------|------|
| `org.openjdk.jol:jol-core` | 객체 메모리 레이아웃 측정 |
| `org.ow2.asm:asm`, `asm-tree`, `asm-util` | 바이트코드 조작 |
| `org.openjdk.jmh:jmh-core`, `jmh-generator-annprocess` | 마이크로벤치마크 |
| `org.junit.jupiter:junit-jupiter` | 단위 테스트 |
| `org.assertj:assertj-core` | 검증 표현 |

## 독립 개념 실습

책 챕터에 직접 묶이지 않거나, 빌드 도구 없이 실행해야 학습 목적이 드러나는 실습은 루트의 독립 폴더에 둡니다.

| 실습 | 실행 방식 | 다루는 내용 |
|------|-----------|-------------|
| [`classpath-basics`](./classpath-basics/) | `./classpath-basics/scripts/run.sh all` | 패키지 경로, `javac -d`, JAR 구조, 컴파일·런타임 클래스패스 |
| [`spi-demo`](./spi-demo/) | `./gradlew :spi-demo:run` | `ServiceLoader`, `META-INF/services`, 런타임 구현체 탐색 |

---

## Troubleshooting Java 실습 앱 (tsj/)

『Troubleshooting Java, 2nd Edition』(Laurențiu Spilcă, Manning) 저자 제공 실습 앱을 `tsj/` 디렉토리에 Maven 원본 구조 그대로 보존한다. Gradle 멀티모듈과는 독립적으로 `mvn` 명령으로 실행한다.

```bash
# 순수 Java 앱
cd tsj/da-ch1-ex1 && mvn compile exec:java -Dexec.mainClass="..."

# Spring Boot 앱
cd tsj/da-ch4-ex1 && mvn spring-boot:run
```

상세 앱↔노트 매핑은 [`tsj/README.md`](./tsj/README.md) 참조.
