# 람다는 invokedynamic으로 컴파일된다 — 바이트코드로 보는 증거

> ch03 03-03 노트(동적 타입 언어 지원과 invokedynamic)의 Phase 3 실습.
> 람다 한 줄이 익명 클래스 `new`가 아니라 `invokedynamic` 호출 지점으로 컴파일되는지,
> `javac` → `javap -c -v` 로 바이트코드·상수풀·BootstrapMethods를 직접 확인한다.

## 관련 이론
- [03-03. 동적 타입 언어 지원과 invokedynamic](../../../ch03_class-loading-mechanism/03-03.동적 타입 언어 지원과 invokedynamic.md)

## 실습 대상
- `LambdaTest.java` — `Runnable r = () -> System.out.println("hello");` 한 줄.
- JDK: major version 65 = Java 21 (javap 출력 `major version: 65`).

## ① main 바이트코드 — 람다 자리에 invokedynamic

`java LambdaTest` 출력:
```
hello
```

`javap -c -v LambdaTest` 의 main:
```
 0: invokedynamic #7,  0   // InvokeDynamic #0:run:()Ljava/lang/Runnable;
 5: astore_1
 6: aload_1
 7: invokeinterface #11,  1   // InterfaceMethod java/lang/Runnable.run:()V
12: return
```
- 람다 `() -> ...` 생성이 익명 클래스 `new`가 아니라 **`invokedynamic` 호출 지점 하나**로 컴파일됐다.
- 반환 타입 `()Ljava/lang/Runnable;` — 인자 없이 `Runnable` 한 개를 만들어 돌려준다.
- 끝의 `, 0` 은 invokedynamic 특유의 **빈 피연산자 자리**. 다른 invoke 명령과 달리 대상 메서드를 바이트코드에 직접 적지 않는다.
- 만든 Runnable을 `r`에 담고(`astore_1`) 그 위에서 `run()`을 부른다(`invokeinterface`). **run() 호출 자체는 컴파일 때 시그니처가 고정된 정적 타입 호출**이다.

## ② BootstrapMethods — 부트스트랩은 LambdaMetafactory

파일 끝:
```
BootstrapMethods:
  0: #43 REF_invokeStatic java/lang/invoke/LambdaMetafactory.metafactory:(...)CallSite
    Method arguments:
      #39 ()V
      #40 REF_invokeStatic LambdaTest.lambda$main$0:()V
      #39 ()V
```
- 이 invokedynamic 지점을 무엇으로 연결할지 정하는 부트스트랩 메서드가 **`LambdaMetafactory.metafactory`**.
- 첫 호출 때 한 번 돌아 `CallSite`를 만들고, 이후 호출은 그 결과에 직접 바인딩(03-03 §2 CallSite 생명주기).
- 인자 3개 = 함수형 인터페이스 메서드 시그니처(`()V`) / 실제 람다 본문 핸들(`lambda$main$0`) / 구현 시그니처(`()V`).

## ③ 람다 본문 — private 합성 메서드로 분리

상수풀:
```
#35 = Utf8         lambda$main$0
#40 = MethodHandle 6:#41   // REF_invokeStatic LambdaTest.lambda$main$0:()V
```
- `System.out.println("hello")` 본문이 `lambda$main$0` 이라는 `private static` 합성 메서드로 빠졌다.
- 부트스트랩이 만드는 CallSite는 이 메서드를 가리키는 핸들을 품는다.

## 배운 점 (이론 ↔ 실습 연결)

- **람다 = invokedynamic + LambdaMetafactory**: 소스에선 함수형 인터페이스 구현처럼 보이지만, 바이트코드는 익명 클래스 생성이 아니다.
- **invokedynamic을 쓴다 ≠ 자바가 동적 타입**: LambdaMetafactory가 만드는 CallSite는 `ConstantCallSite`라 첫 호출에 한 번 묶이면 안 바뀐다. 시그니처가 `()V → Runnable`로 고정. JRuby/Groovy는 *같은* invokedynamic의 부트스트랩 안에 "런타임 타입 디스패처"를 넣어 진짜 동적 디스패치를 만드는 것이고, 자바는 "고정 시그니처 람다 팩토리"를 넣는다 — 같은 틀, 다른 채움.
- **왜 invokedynamic을 빌려 썼나**: 동적 타입을 위해서가 아니라, 익명 클래스를 컴파일 때마다 `.class`로 찍지 않고 런타임에 한 번 만들어 재사용하려고.

## 확장 (챕터 범위 밖, 검증함)
- **문자열 결합**: JDK 9부터 `+` 결합도 invokedynamic으로 컴파일, 부트스트랩은 `StringConcatFactory` (Oracle javadoc, Since 9). 결합 형태가 컴파일 때 정해져 호출 메서드가 런타임에 안 바뀐다.
- **GraalVM Native Image**: invokedynamic 일반은 폐쇄형 세계 가정 때문에 미지원이나, `javac`가 만드는 람다·문자열 결합용 invokedynamic은 *호출 메서드를 런타임에 바꾸지 않으므로* 지원(GraalVM 공식 문서). 제약은 진짜 동적 용법에만.

## 비고
- `*.class` 는 컴파일 산출물이라 Drive·git 에 올리지 않는다(소스·기록만).
- 재현: `cd ~/jvm-practice/ch08-execution-engine/method-dispatch && javac LambdaTest.java && java LambdaTest && javap -c -v LambdaTest`
