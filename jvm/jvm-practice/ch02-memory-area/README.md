# 2장 — 자바 메모리 영역과 메모리 오버플로 실습

책 §2.4의 7개 OOM 종류를 각각 격리된 Gradle 서브모듈로 분리했다. 한 모듈은 *한 OOM만* 재현한다.

## 모듈 ↔ 책의 절 매핑

| 모듈 | 책 위치 | 다루는 OOM | 핵심 JVM 옵션 |
|------|--------|-----------|---------------|
| `heap` | §2.4.1 p.77 | `Java heap space` | `-Xms20m -Xmx20m` |
| `jvm-stack` | §2.4.2 p.81 | `StackOverflowError` | `-Xss128k` |
| `native-stack` | §2.4.2 p.85 | `unable to create native thread` | `-Xss2m` (주의: OS 자원 소모) |
| `method-area` | §2.4.3 p.89 | `Metaspace` | `-XX:MaxMetaspaceSize=10m` |
| `constant-pool` | §2.4.3 p.87 | `Java heap space` (JDK 7+ 변경) | `-Xmx10m` |
| `direct-memory` | §2.4.4 p.91 | `Unable to allocate ... bytes` | `-XX:MaxDirectMemorySize=10m` |
| `layout` | §2.3.2 p.73 | (OOM 아님) JOL로 객체 헤더·인스턴스·패딩 출력 | — |

## 실행

각 모듈은 `application` 플러그인이 적용돼 있고 JVM 옵션이 `build.gradle.kts`에 *고정 박제*돼 있다.

### 안전 실행 가능 (5개) — 본 저장소에서 동작 검증 완료

```bash
cd write/01_language/java/05_JVM/_practice

./gradlew :ch02-memory-area:heap:run                # Java heap space (검증 완료)
./gradlew :ch02-memory-area:jvm-stack:run           # StackOverflowError, stack length 출력 (검증 완료)
./gradlew :ch02-memory-area:method-area:run         # Metaspace OOM (검증 완료)
./gradlew :ch02-memory-area:constant-pool:run       # Java heap space, JDK 7+ 변경 동작 (검증 완료)
./gradlew :ch02-memory-area:layout:run              # JOL 객체 레이아웃 (OOM 없음, 검증 완료)
```

각 모듈은 한 OOM이 나면 *Gradle task가 실패한 것처럼* 보이는데 (`BUILD FAILED`), 그게 *예상 결과*다. OOM 메시지가 stderr에 정상적으로 출력되면 검증 통과.

### 시스템 위험 — 실행 금지 또는 격리된 환경에서만

```bash
# direct-memory: -XX:MaxDirectMemorySize=10m 옵션이 Unsafe.allocateMemory 직접 호출에는
#   적용되지 않아 시스템 RAM을 끝없이 잡을 수 있다. 운영 머신에서 실행 금지.
#   대안: ByteBuffer.allocateDirect(_1MB) 로 변경하면 한계가 먹는다 (책 변형 예제).
./gradlew :ch02-memory-area:direct-memory:run        # 실행 위험 — 코드 박제 용도

# native-stack: 스레드 폭주로 OS 한계까지 스레드를 만들어 시스템을 hang 시킬 수 있다.
#   ulimit -u 로 제한된 셸이나 격리된 컨테이너에서만 실행.
./gradlew :ch02-memory-area:native-stack:run         # 실행 위험 — 코드 박제 용도
```

두 모듈은 *책 §2.4의 코드를 그대로 박제*하는 것이 일차 목적이며, 실행은 환경이 안전할 때만 사용자가 의식적으로 결정한다.

## 책 기준 JDK 12와 본 실습 JDK 21의 차이

| 영역 | 책 (JDK 12) | 본 실습 (JDK 21) |
|------|------------|------------------|
| 메서드 영역 OOM | CGLib `Enhancer.create()` | ByteBuddy `subclass().make().load()` (CGLib는 JDK 17+ 호환성 약함) |
| 런타임 상수 풀 | PermGen 메시지(JDK 6 기준 책 설명) | `String.intern()`이 자바 힙에 객체를 두므로 `Java heap space` |
| 그 외 | 동일 | 동일 |

각 모듈의 `// JDK 21 변경 사유` 주석에서 더 자세히 다룬다.
