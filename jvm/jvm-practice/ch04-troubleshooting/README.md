# 4장 — 가상 머신 성능 모니터링과 문제 해결 도구 실습

책 §4.3~§4.4의 시각화·플러그인 도구를 *관찰 대상 프로그램*으로 박제한다. 4.2 명령줄 도구(jps·jstat·jmap·jstack)는 별도 데모가 필요 없어 모듈로 두지 않고, 이미 떠 있는 다른 챕터 프로세스(예: `ch03-gc:allocation`)에 붙여 실습한다.

## 모듈 ↔ 책의 절 매핑

| 모듈 | 책 위치 | 관찰 도구 | 비고 |
|------|--------|-----------|------|
| `monitoring` | §4.3.2 | JConsole / JMC | CPU 점유 + 데드락 데모. 무한 루프라 관찰 후 수동 종료 |
| `btrace-target` | §4.3.3 | VisualVM BTrace 플러그인 | `add(int,int)` 를 재시작 없이 동적 추적할 대상 |
| `hsdis` | §4.4 | HSDIS 디스어셈블러 | `Bar.sum()` 의 JIT 기계어 출력. HSDIS 라이브러리 필요 |

## 실행 주의

`monitoring` 과 `btrace-target` 은 *의도적으로 멈추지 않는다*(무한 루프·wait). 관찰용으로 띄운 뒤 직접 종료한다. 컴파일만 확인하려면 `run` 대신 `compileJava` 를 쓴다.

```bash
# JConsole 관찰: 띄운 뒤 jconsole 로 attach → 스레드 탭 / Detect Deadlock
./gradlew :ch04-troubleshooting:monitoring:run        # 관찰 후 Ctrl+C

# BTrace 관찰: 띄운 뒤 VisualVM BTrace 플러그인으로 attach
./gradlew :ch04-troubleshooting:btrace-target:run     # 관찰 후 Ctrl+C

# HSDIS: 디스어셈블 출력(HSDIS 라이브러리가 JDK 에 있어야 어셈블리로 풀림)
./gradlew :ch04-troubleshooting:hsdis:run

# 컴파일만 확인(멈추는 데모를 띄우지 않고 골격 검증)
./gradlew :ch04-troubleshooting:monitoring:compileJava
```

## 노트 연결

- 본편: [`../../ch04_troubleshooting/02-01.기본 문제 해결 도구 — 명령줄 도구.md`](../../ch04_troubleshooting/02-01.기본%20문제%20해결%20도구%20—%20명령줄%20도구.md), [`../../ch04_troubleshooting/02-02.시각화 문제 해결 도구.md`](../../ch04_troubleshooting/02-02.시각화%20문제%20해결%20도구.md)
