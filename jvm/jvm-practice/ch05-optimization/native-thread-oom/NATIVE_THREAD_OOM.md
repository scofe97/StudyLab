# native thread OOM 실습 — "힙은 멀쩡한데 터지는 OOM"

> 04-01 노트(최적화 사례 분석) §4의 Phase 3 실습. `OutOfMemoryError`가 *항상 힙 부족이 아님*을
> 직접 재현한다. 힙은 13%밖에 안 쓰는데 *네이티브(스레드)가 먼저 말라* OOM이 나는 장면을 실물로 본다.

## 관련 이론
- [04-01. 최적화 사례 분석](../../../Library/CloudStorage/.../04-01.최적화%20사례%20분석.md) §4 — native thread·메타스페이스 OOM
  (Drive: `write/01_language/java/05_JVM/ch02_automatic-memory-management/04-01.최적화 사례 분석.md`)

## 실습 대상
- `NativeThreadOOM.java` — `while(true)`로 스레드를 끝없이 만들고, 각 스레드를 `Thread.sleep(Long.MAX_VALUE)`로
  *살려둬* 네이티브 스택 자리를 점유한다. 죽지 않으니 OS 스레드 한도까지 쌓인다.
- VM 옵션: `-Xmx256m` — 힙을 넉넉히 잡아 "힙이 아니라 네이티브가 먼저 마른다"를 도드라지게.
- JDK: Temurin 21.0.3 (Java 25 금지 규칙 준수)
- 실행: `java -Xmx256m -cp _run org.runners.jvm.ch05.nativethread.NativeThreadOOM`

## 관측 결과 — 힙은 비었는데 OOM (핵심 실증)

### ① OOM 스택 — 콜론 뒤 영역 표기가 "native thread"
```
[0.572s][warning][os,thread] Failed to start the native thread for java.lang.Thread "Thread-4075"
Exception in thread "main" java.lang.OutOfMemoryError: unable to create native thread:
  possibly out of memory or process/resource limits reached
	at java.base/java.lang.Thread.start0(Native Method)
	at NativeThreadOOM.main(NativeThreadOOM.java:21)
```
- `created threads = 4000`까지 찍히고 **Thread-4075**에서 막혔다.
- `OutOfMemoryError` 뒤 표기가 `Java heap space`가 *아니라* `unable to create native thread`다 —
  진단의 첫 갈림길은 바로 이 *콜론 뒤 영역 이름*을 읽는 것. 힙 부족과 처방이 정반대로 갈린다.

### ② jstat -gcutil — 힙은 텅 비어 GC가 한 번도 안 돌았다
누수가 의심될 때처럼 진단 도구로 붙어 보니(03-01의 jstat), 힙 쪽은 완전히 멀쩡했다.
```
  E      O     YGC   FGC   CGC
13.04   0.00    0     0     0     ← Eden 13%, Old 0%, GC 단 한 번도 안 돔
```
- 스레드가 4000개 넘게 쌓이는 동안 **힙은 13%, GC 횟수 0**. 힙 압박이 전혀 없다.
- 이 숫자가 "힙을 키워봐야 소용없다 — 마르는 건 힙이 아니다"를 못박는다.
  (만약 힙 부족이었다면 O가 차오르고 FGC가 폭발했을 것 — 03-01 LeakDemo의 `255M->255M`처럼.)

### ③ jstack — 살아있는 스레드 수
```
$ jstack <pid> | grep -c '"Thread-'
4074     ← 살려둔 스레드가 그대로 쌓여 있음
```
`Thread.sleep(Long.MAX_VALUE)`로 안 죽게 한 스레드가 4074개 떠 있다 = 네이티브 스택 자리 소진.

## 배운 점 (이론 ↔ 실습 연결)

- **OOM ≠ 힙 부족**: 힙 13%·GC 0회인데 OOM이 났다. 콜론 뒤 `unable to create native thread`를
  읽는 것이 진단의 출발점. 영역 표기만 보고도 "힙을 키울 일이 아니다"를 안다.
- **제로섬**: `-Xmx256m`으로 힙을 잡아둔 채 스레드가 ~4075개에서 막혔다. 한 프로세스의 주소 공간은
  고정 파이라, 힙을 *더* 키웠다면 네이티브 몫이 줄어 *더 빨리* 막혔을 것. 처방은 힙을 *줄이거나* `-Xss`를 줄이는 것 — 힙을 키우는 본능과 정반대.
- **같은 도구, 다른 결론**: 03-01에서 jstat O 우상향=누수였는데, 여기선 jstat가 *멀쩡함*을 보여줌으로써
  "범인은 힙이 아니다"를 *배제*하는 데 쓰였다. 진단 도구는 범인을 찾는 만큼 *무죄를 입증*하는 데도 쓴다.

## 실행 함정 메모 (실습 중 만난 것)

- **파이프 버퍼 갇힘**: `java ... | tail`로 띄우면 stdout이 파이프 버퍼에 갇혀 OOM이 나도 화면에 안 보인다.
  → 파일로 직접 리다이렉트(`> log 2>&1`)해야 깔끔히 잡힌다.
- **`ulimit -u`를 너무 낮추면 JVM이 못 뜬다**: 시스템 보호로 `ulimit -u 400`을 줬더니
  `fork failed: resource temporarily unavailable` — JVM을 *띄우는 fork 자체*가 막혔다.
  한도를 막지 말고 자연 한도까지 가되 *타임아웃 강제 종료*로 시스템을 보호하는 편이 안전.

## 직접 실습하는 법 (재현 절차)

⚠️ OS 스레드 한도를 건드린다. 격리 환경(개인 머신)에서만, 관찰 후 즉시 종료한다.

### 0. 환경 (매 터미널마다)
```bash
export JAVA_HOME=/Users/user/Library/Java/JavaVirtualMachines/temurin-21.0.3/Contents/Home
export PATH=$JAVA_HOME/bin:$PATH
cd ~/jvm-practice/ch05-optimization/native-thread-oom
```

### 1. 컴파일
```bash
javac -d _run src/main/java/org/runners/jvm/ch05/nativethread/NativeThreadOOM.java
```

### 2. 백그라운드 실행 — 파일로 직접 리다이렉트
> ⚠️ `| tail` 같은 파이프로 띄우면 출력이 버퍼에 갇혀 OOM이 안 보인다. 꼭 `> log 2>&1`.
```bash
java -Xmx256m -cp _run org.runners.jvm.ch05.nativethread.NativeThreadOOM > native-oom.log 2>&1 &
```
Enter 치면 `[1] 12345`처럼 PID가 바로 나온다 (`echo "PID=$!"`는 안 쳐도 됨).

### 3. 살아있는 동안 진단 (OOM이 2~5초만에 나니 빠르게)
PID를 손으로 안 넣고 자동으로 잡는 한 줄:
```bash
PID=$(jps | grep -i native | awk '{print $1}'); echo "PID=$PID"; jstat -gcutil $PID; jstack $PID | grep -c Thread-
```

### 4. OOM 스택 확인
```bash
grep -A5 "OutOfMemoryError" native-oom.log
grep "created threads" native-oom.log | tail -1
```

### 5. 정리 (필수 — 안 하면 머신 불안정)
```bash
pkill -9 -f NativeThreadOOM
```

### 실습 중 만난 셸 함정 — `dquote>`
2번 명령을 두 줄로 한꺼번에 붙여넣다 `echo "PID=$!"`의 닫는 따옴표가 잘리면, zsh가
`dquote>`(따옴표 안 닫힘) 프롬프트로 멈춘다. **`Ctrl+C`로 취소**하고, 명령을 *한 줄씩 나눠서*
입력하면 된다. 애초에 `echo "PID=$!"`는 PID를 다시 보여주는 것뿐이라 생략해도 무방하다.

## 비고
- `_run/`(컴파일 산출물)·`native-oom.log`는 실행 산출물이라 Drive에 올리지 않는다(로컬에만).
- ⚠️ 이 데모는 OS 스레드 한도를 건드린다. 격리 환경에서만, 관찰 후 즉시 종료(`pkill -9 -f NativeThreadOOM`).
- 재현: `javac -d _run src/.../NativeThreadOOM.java && java -Xmx256m -cp _run org...NativeThreadOOM > log 2>&1 &` 후 40초 내 OOM. jstat/jstack은 살아있는 동안 붙는다.
