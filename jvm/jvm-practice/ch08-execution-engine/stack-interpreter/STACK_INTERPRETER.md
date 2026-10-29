# 스택 기반 인스트럭션 셋 — 바이트코드로 보는 피연산자 스택

> ch03 03-04 노트(스택 기반 해석 실행 엔진)의 Phase 3 실습.
> `javac` → `javap -c` 로, 모든 계산이 레지스터 없이 피연산자 스택을 밀고(push) 당겨(pop)
> 이뤄지는지, 그리고 `istore` 왕복이 덧셈 비용이 아니라 초기화 비용임을 직접 확인한다.

## 관련 이론
- [03-04. 스택 기반 해석 실행 엔진](../../../ch03_class-loading-mechanism/03-04.스택 기반 해석 실행 엔진.md)

## 실습 대상
- `Calc.java` — `calc()` ((a+b)*c) + `addTwo(int,int)` + `mul(int,int)`.
- JDK: javac 25 (단순 산술이라 버전 무관, 바이트코드 동일).

## ① calc() — (a+b)*c 가 스택을 밀고 당긴다

`java Calc` → `90000`. `javap -c` 의 calc:
```
 0: bipush 100      // 100 push
 2: istore_1        // pop → slot1(a)
 3: sipush 200      // 200 push
 6: istore_2        // pop → slot2(b)
 7: sipush 300      // 300 push
10: istore_3        // pop → slot3(c)
11: iload_1         // a push          → [100]
12: iload_2         // b push          → [100, 200]
13: iadd            // 두 값 pop, 합 push → [300]
14: iload_3         // c push          → [300, 300]
15: imul            // 두 값 pop, 곱 push → [90000]
16: ireturn         // top 반환
```
- 본문 §3 추적표와 **완전 일치**. 11~15가 핵심 — iload 둘로 피연산자를 쌓고, iadd/imul 이 두 값을 pop 해 결과를 push.
- 변수 초기화(0~10)는 `push(bipush/sipush) ↔ pop(istore)` 왕복.

## ② iadd 는 "b를 더하는" 명령이 아니다

`iadd` 바로 앞에 `iload_1`, `iload_2` 가 **둘** 온다(11~13). iadd 는 피연산자를 자기가 들고 오지 않고, *스택에 이미 올라온 두 값을 pop 해 합을 push* 할 뿐이다. 그래서 더하기 전에 iload 둘이 반드시 선행한다. imul 도 동일.

## ③ istore 왕복의 정체 — 덧셈 비용이 아니라 초기화 비용

같은 `a + b` 인데 인자로 받으면 istore 가 사라진다.
```
addTwo(int,int):           calc() 의 a+b 부분:
 0: iload_1                 0: bipush 100 / 2: istore_1   ← 초기화 왕복
 1: iload_2                 3: sipush 200 / 6: istore_2   ← 초기화 왕복
 2: iadd                   11: iload_1 / 12: iload_2
 3: ireturn               13: iadd
```
- `addTwo` 는 인자가 *이미 slot1·slot2 에 들어와 있어서* istore 왕복 없이 `iload, iload, iadd, ireturn` 4줄뿐.
- 즉 calc() 의 `bipush/sipush → istore` 왕복은 **상수를 지역변수에 넣는 초기화** 때문이지, 덧셈 자체의 비용이 아니다. "스택 기반은 명령어가 많다"를 말할 때 이 구분이 중요하다.

## ④ 레지스터 번호가 어디에도 없다

calc·addTwo·mul 어느 메서드에도 `eax` 같은 레지스터 지정이 없다. 모든 연산이 스택 top 근처에서 일어난다. x86 의 `mov eax,100 / add eax,200`(2명령, 레지스터 명시)과 대조하면, 스택 기반은 같은 덧셈을 `iload/iload/iadd`(3명령, 레지스터 없음)로 푼다. 명령어가 더 많은 대신, 이 바이트코드는 레지스터 수가 다른 어떤 CPU 위에서도 동일하게 돈다(이식성). 그 속도 손실은 JIT 가 레지스터 기반 기계어로 바꿔 메운다.

## 배운 점 (이론 ↔ 실습 연결)

- **스택 기반 = 레지스터 미지정**: 계산은 피연산자 스택 top 에서만. 명령에 레지스터 번호 없음.
- **iadd/imul = 스택의 두 값 pop, 결과 push**: 피연산자를 명령이 들고 오지 않음. iload 선행 필수.
- **istore 왕복 ≠ 산술 비용**: 초기화(상수→변수) 때문. addTwo 대조로 확인.
- **명령어 수 많음 ↔ 이식성**: 레지스터 기반 2명령을 스택 기반은 3+명령으로. 대가로 CPU 무관. JIT 가 속도 보완.

## 비고
- `*.class` 는 컴파일 산출물이라 Drive·git 에 올리지 않는다(소스·기록만).
- 재현: `cd ~/jvm-practice/ch08-execution-engine/stack-interpreter && javac Calc.java && java Calc && javap -c Calc`
