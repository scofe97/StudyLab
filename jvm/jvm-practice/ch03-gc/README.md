# 3장 — 가비지 컬렉터와 메모리 할당 전략 실습

책 §3.5~§3.8을 GC 종류별 + 할당 데모로 분리한다. 한 모듈은 *하나의 GC 옵션*을 박제하고, 같은 워크로드(`common/AllocationWorkload`)를 다른 GC로 돌려 비교한다.

## 모듈 ↔ 책의 절 매핑

| 모듈 | 책 위치 | GC 옵션 | 검증 상태 |
|------|--------|--------|---------|
| `common` | (공통) | 워크로드만 제공, GC 옵션 없음 | 라이브러리 |
| `serial` | §3.5.1 | `-XX:+UseSerialGC` | 단일 스레드 STW |
| `parallel` | §3.5.4 | `-XX:+UseParallelGC` | 멀티스레드 STW, throughput 우선 |
| `cms` | §3.5.6 | `-XX:+UseConcMarkSweepGC` | JDK 14 제거됨 — 박제용 |
| `g1` | §3.5.7 | `-XX:+UseG1GC` | JDK 9+ 기본 |
| `zgc` | §3.6.2 | `-XX:+UseZGC` | JDK 15+ stable, JDK 21 Generational |
| `shenandoah` | §3.6.1 | `-XX:+UseShenandoahGC` | Temurin·OpenJDK 일부 포함 |
| `allocation` | §3.8 | 다섯 할당 규칙 데모 | TLAB·Pretenure·Tenuring |

## 안전 실행

각 GC 모듈은 *짧은 워크로드*만 돌린다. OOM을 의도하지 않으므로 시스템 위험이 낮다.

```bash
cd write/01_language/java/05_JVM/_practice

./gradlew :ch03-gc:serial:run        # Serial GC
./gradlew :ch03-gc:parallel:run      # Parallel GC
./gradlew :ch03-gc:g1:run            # G1 GC
./gradlew :ch03-gc:zgc:run           # ZGC
./gradlew :ch03-gc:shenandoah:run    # Shenandoah (Temurin 21)
./gradlew :ch03-gc:cms:run           # JDK 21에서 경고 후 G1으로 대체
./gradlew :ch03-gc:allocation:run    # 할당 규칙 데모
```

## GC 로그 파일

각 모듈은 `-Xlog:gc*=info:file=build/gc.log` 로 GC 로그를 떨군다. 실행 후 `<module>/build/gc.log` 에서 확인.

## 비교 방법

같은 워크로드를 다른 GC로 돌렸을 때 다음 지표를 비교한다:

| 지표 | 어디서 보나 |
|------|------------|
| 총 실행 시간 | Gradle 출력의 `BUILD SUCCESSFUL in <N>s` |
| GC 호출 횟수 | `gc.log` 의 `[gc]` 라인 수 |
| 최대 일시 정지 | `gc.log` 의 `Pause` 라인의 최대값 |
| Throughput | 1 - (GC 시간 / 총 시간) |

## JDK 21 호환 메모

| 모듈 | 호환성 |
|------|------|
| `serial`, `parallel`, `g1`, `zgc` | JDK 21에서 정상 동작 |
| `shenandoah` | Temurin 21 OK, Oracle JDK 21 미포함 |
| `cms` | JDK 14 제거. JDK 21에서 옵션 주면 *경고 출력 후 G1으로 대체*. 코드 박제 의의는 *역사적 학습* |
