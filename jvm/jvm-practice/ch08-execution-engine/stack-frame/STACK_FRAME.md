# 스택 프레임 실습 — Code 속성 + slot 재사용 GC 함정

> ch03 03-01 노트(런타임 스택 프레임 구조)의 Phase 3 실습. ①javap -v 로 max_stack/max_locals 가
> .class 에 박힌 것을 보고, ②slot 재사용 여부가 GC 회수를 가르는 것을 GC 로그로 실증한다.

## 관련 이론
- [03-01. 런타임 스택 프레임 구조](../../../03-01.런타임 스택 프레임 구조.md) §1~2

## 실습 대상
- `FrameSize.java` — static/인스턴스/long 메서드로 stack·locals 칸 수 비교.
- `SlotGc.java` — keep/reuse/null 세 모드로 slot 재사용 GC 영향 비교.
- JDK: Temurin 21.0.3.

## ① Code 속성 — stack/locals 는 칸 개수 (javap -v)

```
$ javac FrameSize.java && javap -v FrameSize.class
static int add(int, int);     stack=2, locals=2   ← static: this 없음
int instanceAdd(int, int);    stack=2, locals=3   ← slot0=this 포함 → locals +1
static long withLong(long,int); stack=4, locals=5 ← long 이 slot 2칸
```
- `locals`·`stack` 의 숫자는 *바이트가 아니라 칸* 개수다. 32비트 이하=1칸, long/double=2칸.
- `withLong`: x(long, slot 0~1) + y(int, slot 2) + z(long, slot 3~4) = **5칸** → locals=5.
  바이트로는 8+4+8=20B 지만 칸으론 5.
- 이 값들이 *컴파일된 .class 에 숫자로 박혀* 있다 = 프레임 크기가 컴파일 때 확정됨의 증거.

## ② slot 재사용 GC 함정 — 같은 64MB, 회수가 갈린다

`java -Xlog:gc -Xmx256m SlotGc <mode>` 실측:
```
keep   : Pause Full (System.gc()) 66M->65M(224M)   ← 회수 안 됨 (slot 이 placeholder 참조 보유)
reuse  : Pause Full (System.gc()) 66M->0M(8M)       ← 회수됨 (int a=0 으로 slot 덮음)
null   : Pause Full (System.gc()) 66M->0M(8M)       ← 회수됨 (placeholder=null 로 끊음)
```
- `keep`: 블록을 벗어나 scope 는 끝났지만 그 slot 을 *다른 변수가 재사용하지 않아* placeholder
  참조를 그대로 들고 있다. 지역 변수 테이블(slot)이 GC root 라, root 에서 도달 가능 → 안 회수.
  → `66M->65M` (안 줄어듦).
- `reuse`: `int a = 0` 이 placeholder 의 slot 을 덮어써 참조가 끊긴다 → `66M->0M` (다 회수).
- `null`: `placeholder = null` 로 명시적으로 끊어도 회수 → `66M->0M`.
- 본질은 *slot 재사용*. 일반 코드는 이후 변수가 자연히 slot 을 덮어 회수되므로, 명시적 null 은
  *재사용이 안 일어나는 특수 상황*에서만 의미 있다.

## 배운 점 (이론 ↔ 실습 연결)

- **프레임 크기는 컴파일 확정**: javap 의 stack=/locals= 가 .class Code 속성에 박힌 숫자.
- **단위는 칸**: int(4B)도 1칸, long(8B)도 2칸. this·long 이 locals 를 늘린다.
- **slot=GC root, 재사용이 회수를 가름**: keep(66M->65M) vs reuse(66M->0M)를 GC 로그로 실증.
  null 은 특수 케이스, 본질은 slot 재사용.

## 비고
- `*.class` 는 컴파일 산출물이라 Drive 에 올리지 않는다(소스·기록만).
- 재현: `cd ~/jvm-practice/ch08-execution-engine/stack-frame && javac *.java && javap -v FrameSize.class` / `java -Xlog:gc -Xmx256m SlotGc keep|reuse|null`.
