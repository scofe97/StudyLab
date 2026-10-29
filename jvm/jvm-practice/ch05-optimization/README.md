# 5장 — 최적화 사례 분석 및 실전 실습

책 §5.2의 장애 사례 중 *단일 JVM에서 재현 가능한 것만* 모듈로 박제한다. 대부분의 5장 사례는 분산 환경(클러스터 동기화 OOM)이나 외부 SW(Eclipse 튜닝)에 묶여 로컬 단일 JVM으로 재현되지 않으므로, 그쪽은 노트 본문(`../../ch05_optimization/`)으로만 다룬다.

## 모듈 ↔ 책의 절 매핑

| 모듈 | 책 위치 | 재현하는 것 | 비고 |
|------|--------|-------------|------|
| `native-thread-oom` | §5.2 | `unable to create new native thread` | 힙은 남는데 네이티브가 먼저 마름. ⚠️ 시스템 부담 |

## 재현 불가 — 노트 본문으로만

| 사례 | 책 위치 | 모듈화 못 하는 이유 |
|------|--------|---------------------|
| 클러스터 간 동기화 OOM | §5.2.2 | 분산 노드·RMI 환경 의존 |
| Eclipse IDE 튜닝 | §5.3 | 외부 SW 대상 (`eclipse.ini` 설정은 노트에 박제) |
| 대용량 힙 Full GC STW | §5.2.1 | 12GB 힙·장시간 관찰 필요, 데모로 부적합 |

## 실행 주의

`native-thread-oom` 은 스레드를 끝없이 만들어 *OS 스레드 한도를 건드린다*. 머신이 일시적으로 불안정해질 수 있어 **격리 환경에서만, 관찰 후 수동 종료**한다. 컴파일만 확인하려면 `run` 대신 `compileJava` 를 쓴다.

```bash
# ⚠️ 격리 환경 전용 — OS 스레드 한도 소진. 운영/공용 머신 금지
./gradlew :ch05-optimization:native-thread-oom:run        # 관찰 후 Ctrl+C

# 컴파일만 확인(데모를 띄우지 않고 골격 검증)
./gradlew :ch05-optimization:native-thread-oom:compileJava
```

## 노트 연결

- 본편: [`../../ch05_optimization/02-01.최적화 사례 분석.md`](../../ch05_optimization/02-01.최적화%20사례%20분석.md), [`../../ch05_optimization/02-02.실전 — Eclipse IDE 튜닝.md`](../../ch05_optimization/02-02.실전%20—%20Eclipse%20IDE%20튜닝.md)
