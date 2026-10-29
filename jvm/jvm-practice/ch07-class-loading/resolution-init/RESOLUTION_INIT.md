# 해석·초기화 실습 — 부모 먼저 + clinit 데드락

> ch03 02-03 노트(해석과 초기화)의 Phase 3 실습. ①부모 <clinit>이 자식보다 먼저 도는 것과
> ②<clinit> 무한 루프가 초기화 락을 영영 안 놓아 생기는 데드락을 직접 재현·진단한다.

## 관련 이론
- [02-03. 해석과 초기화](../../../ch03_class-loading-mechanism/02-03.해석과 초기화.md) §2~3

## 실습 대상
- `ParentFirst.java` — `Parent`(static A=1, static{A=2}) + `Sub`(static B=A). `Sub.B` 출력.
- `ClinitDeadlock.java` — `DeadLoopClass`의 `<clinit>`에 `while(true){}`. 두 스레드가 동시 초기화 시도.
- JDK: Temurin 21.0.3.

## ① 부모 먼저 — Sub.B = 2

```
$ java ParentFirst
2
```
`Sub.B`를 읽는 순간 `Sub` 초기화가 필요한데, 그 전에 부모 `Parent`의 `<clinit>`이 먼저 실행돼
`A=2`가 된다. 그 다음 `Sub`의 `<clinit>`이 `B=A`를 실행하므로 `B=2`. 부모 우선 규칙이 없었다면
`B`는 준비 단계 기본값 0이거나 대입 전 값이었을 것 — JVM이 부모를 앞세워 일관성을 지킨다.

## ② clinit 데드락 — 한 스레드만 돌고 나머지는 초기화 락 대기

```
$ java ClinitDeadlock
Thread[#20,Thread-0,5,main] start
Thread[#21,Thread-1,5,main] start
Thread[#20,Thread-0,5,main] init DeadLoopClass
   ← 여기서 멈춤. "run over" 는 둘 다 안 찍힘 (Ctrl+C 로 종료)
```

### jstack 진단 (실측)
```
"Thread-0" ... cpu=70472.28ms ... runnable
   java.lang.Thread.State: RUNNABLE
        at DeadLoopClass.<clinit>(ClinitDeadlock.java:18)   ← 무한 루프 중, CPU 70초+ 태움

"Thread-1" ... cpu=0.70ms ... waiting on condition
   java.lang.Thread.State: RUNNABLE
        at ClinitDeadlock.lambda$main$0(...:28)
        - waiting on the Class initialization monitor for DeadLoopClass   ← 초기화 락 대기
```

- **Thread-0**: `<clinit>` 의 `while(true)` 에 갇혀 초기화 락을 영영 안 놓음. CPU 70초+ 누적.
- **Thread-1**: `DeadLoopClass` 초기화 락을 기다리며 멈춤(cpu 0.70ms — 거의 안 씀).
- 일반 무한 루프(모든 스레드가 CPU 탐)와 달리, *한 스레드만 RUNNABLE 로 루프 + 나머지는 대기*라
  원인 찾기가 까다롭다.

### 디테일 — Thread-1 이 BLOCKED 아니라 RUNNABLE 로 보인다
대기 중인 Thread-1 의 `Thread.State` 가 `BLOCKED` 가 아니라 **`RUNNABLE` + `waiting on condition`**
으로 나온다. JVM 의 클래스 초기화 락은 일반 `synchronized` 모니터와 메커니즘이 달라(내부적으로
조건 대기 유사) 자바 레벨 상태를 RUNNABLE 로 표시하기도 한다. **진짜 단서는 상태 라벨이 아니라
`- waiting on the Class initialization monitor for ...` 한 줄** — 이게 "초기화 락 대기 = clinit
데드락"의 결정적 지문이다.

## 배운 점 (이론 ↔ 실습 연결)

- **부모 우선이 결과를 바꾼다**: B=2 가 부모 `<clinit>`(A=2)이 먼저 도는 증거. 순서가 없었다면 0/1.
- **초기화 락의 양면**: 같은 락이 싱글턴을 스레드 안전하게 만들면서(<clinit> 1회·1스레드), <clinit>
  안 무한 루프 땐 데드락이 된다.
- **clinit 데드락의 지문**: jstack 에서 한 스레드만 `<clinit>` 에서 RUNNABLE(+CPU 누적) 이고 다른
  스레드는 `Class initialization monitor` 대기. State 라벨이 아니라 이 `waiting on` 줄로 식별.

## 비고
- `*.class` 는 컴파일 산출물이라 Drive 에 올리지 않는다(소스·기록만).
- ② 는 무한 루프라 관찰 후 Ctrl+C 종료. jstack 은 살아있는 동안 다른 터미널에서.
- 재현: `cd ~/jvm-practice/ch07-class-loading/resolution-init && javac *.java && java ParentFirst` / `java ClinitDeadlock`.
