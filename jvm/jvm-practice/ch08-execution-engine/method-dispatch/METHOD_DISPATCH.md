# 메서드 디스패치 실습 — 바이트코드로 보는 정적 vs 동적

> ch03 03-02 노트(메서드 호출 — 해석과 정적·동적 디스패치)의 Phase 3 실습.
> `javac` → `javap -c` 로 정적 디스패치(오버로딩)와 동적 디스패치(오버라이딩)가
> 바이트코드에 어떻게 다르게 나타나는지 직접 확인한다.

## 관련 이론
- [03-02. 메서드 호출 — 해석과 정적·동적 디스패치](../../../03-02.메서드 호출 — 해석과 정적·동적 디스패치.md)

## 실습 대상
- `StaticDispatch.java` — 오버로딩 3버전. 정적 타입으로 컴파일 때 버전 결정.
- `DynamicDispatch.java` — 오버라이딩. 실제 타입으로 실행 중 버전 결정.
- JDK: Temurin 21 (실행 확인은 Java 25, 단순 실행이라 버전 무관).

## ① 정적 디스패치 — 시그니처가 컴파일 때 박힌다

`java StaticDispatch` 출력:
```
hello, guy!
hello, guy!
```
`man`·`woman` 의 실제 타입은 Man·Woman 이지만 둘 다 `hello, guy!` 가 나온다. 정적 타입이 Human 이라서다.

`javap -c StaticDispatch` 의 main:
```
26: invokevirtual #34   // Method sayHello:(LStaticDispatch$Human;)V
31: invokevirtual #34   // Method sayHello:(LStaticDispatch$Human;)V
```
- 두 호출 모두 시그니처가 `(...Human;)V` 로 **컴파일 시점에 고정**돼 있다.
- 실제 인자가 Man·Woman 인데도 `sayHello(Human)` 이 선택됨 = 컴파일러가 *정적 타입* 을 보고 골랐다는 증거.

## ② 동적 디스패치 — 같은 바이트코드, 갈리는 실행

`java DynamicDispatch` 출력:
```
man say hello
woman say hello
woman say hello
```
세 번째는 `man = new Woman()` 으로 같은 변수의 실제 타입만 바꾼 것이라 결과가 woman 으로 바뀐다.

`javap -c DynamicDispatch` 의 main:
```
17: invokevirtual #13   // Method DynamicDispatch$Human.sayHello:()V
21: invokevirtual #13   // Method DynamicDispatch$Human.sayHello:()V
33: invokevirtual #13   // Method DynamicDispatch$Human.sayHello:()V
```
- 세 호출 모두 **똑같은 `Human.sayHello`** (같은 상수 #13) 다.
- 그런데 실행 결과는 man/woman/woman 으로 갈린다 = JVM 이 *실행 중에 수신자의 실제 타입* 을 보고 메서드를 고른다는 증거.

## 배운 점 (이론 ↔ 실습 연결)

- **해석(정적)은 컴파일 때 박힌다**: 오버로딩은 invokevirtual 의 시그니처 자체가 정적 타입으로 고정.
- **디스패치(동적)는 실행 때 결정된다**: 같은 invokevirtual #13 한 줄이 수신자 실제 타입에 따라 다른 구현을 부른다.
- **컴파일러는 어느 구현이 불릴지 모른다**: 바이트코드엔 `Human.sayHello` 만 적히고, 실제 선택은 JVM 의 자식→부모 탐색이 실행 중에 한다.

## 비고
- `*.class` 는 컴파일 산출물이라 Drive 에 올리지 않는다(소스·기록만).
- 재현: `cd ~/jvm-practice/ch08-execution-engine/method-dispatch && javac *.java && java StaticDispatch && java DynamicDispatch && javap -c DynamicDispatch`
