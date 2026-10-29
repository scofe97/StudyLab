# 시각화 모니터링 실습 — JConsole · JFR 로 같은 데이터를 GUI/기록으로

> 03-02 노트(시각화 문제 해결 도구)의 Phase 3 실습. 03-01 이 *명령줄*로 본 GC·스레드를
> 여기서는 *그래프와 기록*으로 본다. 같은 현상을 다른 렌즈로 보며 "왜 GUI 도구가 따로 있나"를 체감한다.

## 관련 이론
- [03-02. 시각화 문제 해결 도구](../../../../write/01_language/java/05_JVM/ch02_automatic-memory-management/03-02.시각화%20문제%20해결%20도구.md)

## 실습 대상
- `VisualMonitorDemo.java` — 한 프로그램에 두 현상을 묶었다.
  - **메모리 톱니**: `OOMObject`(64KB `byte[]`)를 1000개 채우다 `list.clear()+System.gc()`로 급락 → 무한 반복
  - **데드락**: `deadlock-1`/`deadlock-2`가 `lockA`/`lockB`를 엇갈린 순서로 잡아 순환 대기
- 실행: `java -Xms100m -Xmx100m -XX:+UseSerialGC VisualMonitorDemo`
  - 작은 고정 힙 + SerialGC로 톱니가 또렷하게 보인다.
- JDK: Temurin 21.0.3 (Java 25 금지 규칙 준수)

## ① JConsole — 실시간 그래프 + 데드락 버튼 (사용자 GUI 관찰)

### 메모리 탭 — 톱니 그래프
Eden(또는 Heap) 사용량이 차오르다 GC 때마다 급락하는 톱니가 또렷하게 그려졌다.
하단 GC 카운터로 어떤 GC가 도는지까지 읽힌다.
```
Copy                : 5 collections   ← Minor GC (Young 영역, SerialGC의 Young 수집기)
MarkSweepCompact    : 3 collections   ← Full GC (System.gc() 가 부른 것)
```
- `list.clear()` 로 참조를 놓은 뒤 `System.gc()` 가 도니 톱니가 깊게 급락한다.
- `System.gc()` 는 **Full GC**다 — 그 증거가 `MarkSweepCompact` 카운터 증가. (실무에선 코드에 박지 말 것.)

### 스레드 탭 — [Detect Deadlock] 버튼
버튼 한 번에 JVM이 순환 대기를 찾아 데드락 스레드를 짚어줬다 (명령줄 `jstack`의 "Found deadlock"을 버튼으로).
```
deadlock-1  BLOCKED  — lockB(@...) 를 기다림, owned by deadlock-2
deadlock-2  BLOCKED  — lockA(@...) 를 기다림, owned by deadlock-1
```
두 스레드가 서로가 쥔 락을 마주 기다린다 = 순환. 락 주소가 교차하는 것으로 순환을 눈으로 확인했다.

## ② JFR (Java Flight Recorder) — 저오버헤드 블랙박스 기록 (CLI 재현)

JConsole이 *실시간으로 보는* 도구라면, JFR은 *기록해두고 나중에 되감는* 블랙박스다.
JVM이 스스로 이벤트를 링 버퍼에 적어 외부 프로파일러가 끼어들 때보다 오버헤드가 낮다.

기록: `-XX:StartFlightRecording=filename=demo.jfr,duration=...` 로 떠서 `demo.jfr` 생성.
분석: `jfr summary demo.jfr` / `jfr print demo.jfr`.

### jfr summary — 어떤 이벤트가 몇 건 잡혔나
```
Event Type                        Count
jdk.ObjectAllocationSample          11   ← 톱니를 만든 할당 샘플 (누가 객체를 만드나)
jdk.GCHeapMemoryPoolUsage            6
jdk.ExecutionSample                  4   ← 스택 샘플 (CPU가 어디 있나)
jdk.GCPhasePauseLevel1               3   ← GC 멈춤 단계
jdk.GCHeapSummary                    2   ← GC 전후 힙 크기
```
- `jdk.ObjectAllocationSample`(11건) = JConsole 톱니의 *원인*을 이벤트로 본 것. "무엇이 할당되는가"를 jmap `-histo` 없이 기록만으로 안다.
- `jdk.ExecutionSample`(4건) = jstack 스택 스냅숏을 *시간에 걸쳐 여러 번* 찍은 것. `fillHeapForever → Thread.sleep`이 잡혔다.
- `jdk.GCHeapSummary` = GC 전후 힙 크기 (Before ≈ 27.7MB → After ≈ 5.2MB 로 톱니 급락이 수치로).

### 핵심 — 왜 JFR이 따로 있나
JConsole/jstack은 *내가 보는 그 순간*만 안다. JFR은 *지나간 구간 전체*를 링 버퍼에 적어둬서,
장애가 터진 뒤 "그때 무슨 일이 있었나"를 되감을 수 있다. 저오버헤드라 **운영 JVM에 상시 켜둘 수 있는** 게 결정적 차이다.

## 배운 점 (이론 ↔ 실습 연결)

| 03-01 명령줄 | 03-02 시각화 | 같은 현상, 다른 렌즈 |
|--------------|--------------|----------------------|
| `jstat -gcutil` 숫자 추세 | JConsole 메모리 **톱니 그래프** | 숫자로 추세 vs 그래프로 모양 |
| `jstack` "Found deadlock" | JConsole **[Detect Deadlock] 버튼** | 텍스트 스냅숏 vs 버튼 한 번 |
| `jmap -histo` 그 순간 스냅숏 | JFR `ObjectAllocationSample` 기록 | 지금 살아있는 것 vs 시간에 걸친 할당 흐름 |
| (해당 없음 — 실시간만) | JFR 링 버퍼 **되감기** | 지나간 구간을 사후에 본다 |

- **톱니는 누수가 아니다**: `clear()+gc()`로 회수되므로 정상. 03-01 LeakDemo의 static 리스트는 회수 안 돼 누수였다.
  *할당이 많다 ≠ 누수* — 회수되는지가 판별점. (FGC 후 Old가 빠지는가와 같은 논리.)
- `System.gc()` = Full GC. JConsole `MarkSweepCompact` 카운터로 실증.

## 비고
- `demo.jfr`(327KB)·`run.out`은 실행 산출물이라 Drive에 올리지 않는다(~/jvm-practice 로컬에만).
- 재현: `javac VisualMonitorDemo.java && java -Xms100m -Xmx100m -XX:+UseSerialGC VisualMonitorDemo` 후 다른 터미널에서 `jconsole` attach. JFR은 `-XX:StartFlightRecording=filename=demo.jfr` 추가.
- IntelliJ Profiler attach 실습은 생략 — JConsole+JFR로 톱니·데드락·할당 흐름이 모두 관찰됐다.
