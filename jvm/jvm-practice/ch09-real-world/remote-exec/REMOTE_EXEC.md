# 원격 실행 — 상수 풀 치환으로 System 출력 가로채기

> ch03 04-03 노트(실전 — 원격 실행 기능)의 Phase 3 실습.
> 외부 클래스의 `System.out` 출력이 서버 콘솔로 새지 않고, 상수 풀의 `java/lang/System`을
> `HackSystem`으로 바이트 치환해 버퍼로 회수되는 것을 *실제로 실행해* 확인한다.

## 관련 이론
- [04-03. 실전 — 원격 실행 기능 (설계부터 바이트코드 치환까지)](../../../ch03_class-loading-mechanism/04-03.실전 — 원격 실행 기능 설계.md)

## 실습 대상 (6 파일)
- `HelloWorld.java` — 실행 대상. `System.out.println` 두 번.
- `ByteUtils.java` — 바이트↔정수 변환, 부분 교체 (길이 필드 처리).
- `ClassModifier.java` — 상수 풀 Utf8 치환 + 길이 보정.
- `HackSystem.java` — System 대체, out/err을 `ByteArrayOutputStream` 버퍼로.
- `HotSwapClassLoader.java` — `defineClass` 노출, 매번 새 인스턴스.
- `JavaClassExecuter.java` — 입구. 치환→적재→리플렉션 실행→버퍼 반환.
- JDK: javac 25 (단순 실행, 버전 무관).

## ① 동작 — 출력이 콘솔이 아니라 버퍼로

```bash
javac *.java
java JavaClassExecuter
```
출력:
```
=== 버퍼로 회수한 결과 ===
hello
world
=== (위 두 줄이 HelloWorld 의 출력. 직접 콘솔이 아니라 execute 가 반환한 문자열) ===
```
- `hello`/`world`가 `=== 버퍼로 회수한 결과 ===` *아래* 나온다. execute()가 끝나고 `main`이 *반환 문자열*을 찍은 것 → 콘솔로 직접 샌 게 아니라 회수됨.

## ② 치환이 실제로 일어났나 (검증 지표)

`ClassModifier.modifyUTF8Constant("java/lang/System","HackSystem")` 전후:
- `constant_pool_count: 31` (HelloWorld.class)
- 치환 후 바이트 길이: **442 → 436** (6 줄어듦 = `java/lang/System`(16바이트) → `HackSystem`(10바이트))
- 치환본에 `'HackSystem' 포함: true`, `'java/lang/System' 잔존: false`

→ Utf8 항목의 문자열 + length 필드(16→10)를 함께 고치고 뒤 바이트를 6칸 당긴 결과.

## ③ 서버 콘솔 vs 클라(버퍼) 구분 — 결정적 증명

"이 출력이 서버 콘솔로 샌 건가, 버퍼로 회수된 건가?"는 겉보기로 구분 안 된다(둘 다 콘솔에 hello/world). 리트머스: **execute()의 반환값을 안 찍어보기.**

```java
String result = JavaClassExecuter.execute(bytes);   // 실행만, result 안 찍음
// System.out.print(result);  ← 주석
```
결과: **콘솔에 hello/world가 하나도 안 보임** (버퍼 길이=12로 안에는 있음).
- 만약 직접 `System.out`으로 샜다면 result를 안 찍어도 콘솔에 떴어야 한다 → 안 떴으니 **버퍼에 갇힌 것 = 치환 성공.**
- 그 버퍼 내용은 오직 execute()의 *반환 String*으로만 나온다 → 네트워크로 클라에게 보낼 수 있다(04-03 목표).

서버/클라 매핑: `execute()` = 서버(실행), 반환 String을 받는 쪽 = 클라(본문은 JSP/HTTP). 이 실습은 네트워크가 없어 `main`이 클라 역할 겸함.

## 잡은 버그 (실습 중 발견)

`ByteUtils.bytes2Int`를 책 의사코드대로 옮겼더니 `--len` 부작용으로 루프 종료 조건이 망가져 **constant_pool_count를 0으로 읽어** 치환이 한 번도 안 일어났다(출력이 콘솔로 샘). `for (i=start; i<start+len; i++)` 안에서 `n <<= (--len)*8` 가 `len`을 줄여 `start+len` 조건이 함께 줄어든 것.

```java
// 수정 전 (버그): len 을 시프트에 쓰며 감소 → 종료조건 오염
n <<= (--len) * 8;
// 수정 후: len 불변, 별도 shift 계산
for (int i = 0; i < len; i++) { int shift = (len-1-i)*8; sum |= (b[start+i]&0xff) << shift; }
```
교훈: 책/외부 의사코드는 *실제 실행으로 검증* 후 신뢰. 바이트 파싱은 한 칸만 틀려도 조용히 실패한다.

## 핫스왑 (심화)
`HelloWorld.java`의 문자열을 바꿔 `javac HelloWorld.java` 후 다시 `java JavaClassExecuter` 하면 수정본이 반영된다 — JavaClassExecuter가 매번 *새* HotSwapClassLoader로 적재하기 때문(동일성=이름+로더). 같은 로더 재사용이면 캐시된 구버전이 나온다.

## 비고
- `*.class` 는 git·Drive 에 올리지 않는다(소스·기록만).
- 재현: `cd ~/jvm-practice/ch09-real-world/remote-exec && javac *.java && java JavaClassExecuter`
