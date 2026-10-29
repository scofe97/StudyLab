# 누수 진단 실습 — jps → jstat → jmap

> 03-01 노트(명령줄 진단 도구)의 Phase 3 실습. *진단당할 JVM*(`LeakDemo`)을 일부러 띄우고
> 핵심 3도구로 메모리 누수를 추적한다. 노트가 설명한 "추세로 의심 → 스냅숏으로 확정" 순서를 실물로 재현한다.

## 관련 이론
- [03-01. 기본 문제 해결 도구 — 명령줄 도구](../../../../write/01_language/java/05_JVM/ch02_automatic-memory-management/03-01.기본%20문제%20해결%20도구%20—%20명령줄%20도구.md)

## 실습 대상
- `LeakDemo.java` — static `List<byte[]>`에 1MB/라운드씩 누적, remove 없음.
  GC 루트(static)에서 도달 가능하므로 Full GC로도 회수 불가 = 누수.
- 실행: `java -Xmx256m -Xlog:gc:file=gc.log LeakDemo`
- JDK: Temurin 21.0.3 (Java 25 금지 규칙 준수)

## 진단 체인과 관측 결과

### ① jps -l — 프로세스 식별
여러 JVM(Gradle/IDEA/TPS operator) 사이에서 `-l`의 풀네임 덕에 `70805 LeakDemo`를 정확히 골랐다.
다른 모든 도구가 vmid를 인자로 받으므로 진단은 jps로 시작한다.

### ② jstat -gcutil <pid> 250 20 — 누수 추세 관찰
```
  E      O      M     ...   YGC   FGC
 95.65   0.00  ...           0     0     ← Eden 차오름
  6.25   8.30  96.36 ...     1     0     ← Young GC 후 Eden 비워졌으나 O(Old)는 0→8.30%로 올라가 안 내려옴, S1=100%
```
Eden은 GC로 비워지는데 **Old가 우상향하며 회수되지 않는** 패턴 = 누수 초입.
(부하 증가였다면 GC 후 함께 떨어졌어야 한다.)

### 64MB 힙 버전의 gc.log — Full GC 회수 0 증거
```
GC(29) Pause Full (G1 Compaction Pause) 63M->63M(64M)   ← Full GC인데 63M→63M, 회수 0
GC(31) Pause Full (G1 Compaction Pause) 63M->63M(64M)   ← 반복, 여전히 0
```
정상이면 `63M->10M`처럼 떨어진다. mark-compact를 돌려도 안 줄면 누수 확정.

### ③ jmap — 범인 객체 확정
```
jmap -histo:live <pid>
 num   #instances     #bytes  class name
   1:     115837    118302400  [B          ← byte[] 11.5만 개 = 118MB, 압도적 1위 = 범인
```
```
jmap -dump:live,format=b,file=leak.hprof <pid>
Heap dump file created [123607504 bytes in 0.147 secs]
```
118MB 덤프가 **0.147초** 걸렸다 = 그동안 애플리케이션 STW 정지. 수 GB면 수 초로 늘어 운영 부담.
실무에선 이 .hprof를 별도 장비로 옮겨 Eclipse MAT/VisualVM으로 지배 트리·누수 리포트를 본다.

## 배운 점 (Phase 1 ↔ 실습 연결)
- "FGC 빈번 + O 안 내려옴 = 누수"가 gc.log `63M->63M`으로 실증됐다.
- 덤프 STW(0.147s)를 직접 측정 — 가벼운 jstat 먼저, 무거운 jmap은 의심 굳은 뒤 순서의 이유를 체감.
- `-histo`가 덤프 없이 빠르게 범인 클래스를 지목 → 덤프는 더 깊은 분석이 필요할 때.

## 비고
- `leak.hprof`(118MB)·`gc*.log`는 실행 산출물이라 Drive에 올리지 않는다(~/jvm-practice 로컬에만).
- 재현: `javac LeakDemo.java && java -Xmx256m -Xlog:gc:file=gc.log LeakDemo` 후 10초 내 jps→jstat→jmap.
