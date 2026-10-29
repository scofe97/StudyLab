# ch12 · interrupt 플래그 실습 — isInterrupted() vs Thread.interrupted()

**대응 노트**: `05_JVM/ch05_efficient-concurrency/03-01.스레드 생성과 생명주기.md` §4-3
**책**: 《JVM 밑바닥까지 파헤치기》 §12 효율적 동시성

## 무엇을 보는가

두 메서드는 모두 인터럽트 플래그를 **읽는다**. 차이는 읽은 *뒤*에 있다.

- `isInterrupted()` (인스턴스, 멱등 조회) — 읽기만 함. 몇 번 읽어도 `true`.
- `Thread.interrupted()` (static, "읽고 치운다") — 읽는 순간 `false`로 clear. 두 번째 읽기는 `false`.

플래그를 *켜는* 것은 두 조회 메서드가 아니라 인스턴스 메서드 `interrupt()`다.

## 실행

```bash
JAVA_HOME=$(/usr/libexec/java_home -v 21)
$JAVA_HOME/bin/javac InterruptFlagDemo.java
$JAVA_HOME/bin/java InterruptFlagDemo
```

## 출력 (확인된 동작)

```
interrupt() 호출 직후 플래그 = true

[isInterrupted()] 읽고 둔다 — 멱등 조회
  1차 isInterrupted() = true
  2차 isInterrupted() = true  (← 여전히 true, 유지됨)

[Thread.interrupted()] 읽고 치운다 — 읽는 순간 비움
  1차 Thread.interrupted() = true  (← true 반환, 동시에 clear)
  2차 Thread.interrupted() = false  (← false, 이미 비워짐)

clear 후 isInterrupted() = false  (← 인터럽트 흔적 사라짐)
```

마지막 줄이 핵심이다. `Thread.interrupted()`로 비운 뒤 `isInterrupted()`도 `false`가 되므로,
**두 메서드는 같은 하나의 플래그를 본다**. 좌우 분기처럼 보여도 플래그는 하나다.

## 왜 중요한가

인터럽트를 처리하고 플래그를 안 비우면, 뒤이은 `sleep()`이 남은 플래그 탓에
의도치 않은 `InterruptedException`을 또 던진다. 그래서 "확인만"은 `isInterrupted()`,
"처리 끝, 소비"는 `Thread.interrupted()`로 비운다.
