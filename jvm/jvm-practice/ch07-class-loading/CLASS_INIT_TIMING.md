# 클래스 초기화 시점 실습 — 능동 참조 vs 수동 참조

> ch03 02-01 노트(클래스 로딩 시점과 생명주기)의 Phase 3 실습. `static{}` 블록이 찍히는지를
> 관측 지점 삼아, *어떤 코드가 초기화를 일으키고 어떤 코드는 안 일으키는가*를 눈으로 확인한다.

## 관련 이론
- [02-01. 클래스 로딩 시점과 생명주기](../../Library/CloudStorage/.../02-01.클래스 로딩 시점과 생명주기.md) §2~3
  (Drive: `write/01_language/java/05_JVM/ch03_class-loading-mechanism/02-01.클래스 로딩 시점과 생명주기.md`)

## 실습 대상
- `SuperClass.java` — `static{}`에 `SuperClass init!` 출력 + `public static int value = 123`. 자식 `SubClass`도 `static{}` 보유.
- `ConstClass.java` — `static{}`에 `ConstClass init!` 출력 + `static final String HELLO_WORLD = "hello world"` (컴파일 타임 상수).
- `Test1/2/3.java` — 노트 §3 세 예제의 main.
- JDK: Temurin 21.0.3.
- 실행: `javac *.java && java Test1 / Test2 / Test3`

## 관측 결과 — init! 이 찍히는가로 초기화를 본다

### 예제 1 — 자식을 통한 부모 static 접근 (`java Test1`)
```
SuperClass init!     ← 부모만 초기화
123
```
`SubClass init!` 은 **안 찍힌다.** static 필드는 그 필드를 *선언한 클래스*(SuperClass)만 초기화하기 때문.
자식을 거쳐 접근해도 자식은 안 깨어난다 — 직관이 깨지는 자리.

### 예제 2 — 배열 선언 (`java Test2`)
```
배열 선언 완료 (위에 init! 이 안 나왔으면 통과)
```
`SuperClass init!` 이 **안 찍힌다.** `new SuperClass[10]` 은 SuperClass 초기화가 아니라
JVM이 배열 클래스 `[LSuperClass` 를 만드는 동작(anewarray)이다. 원소 타입 초기화와는 별개.

### 예제 3 — 컴파일 타임 상수 (`java Test3`)
```
hello world
```
`ConstClass init!` 이 **안 찍힌다.** static final 상수는 컴파일 때 값이 *Test3 의 상수 풀로 복사*돼,
런타임에 ConstClass 를 들여다볼 일이 없다.

## 바이트코드로 "복사 vs 원본 읽기" 직접 확인

노트 §3의 "복사했나 vs 원본을 깨워야 하나"가 바이트코드에 그대로 드러난다.

### Test3 — 값이 복사됨 (`javap -c -p Test3.class`)
```
3: ldc           #15   // String hello world
```
`ldc "hello world"` — 문자열이 **Test3 안에 직접 박혀** 있다. `getstatic ConstClass.*` 가
**아예 없다.** 그래서 런타임에 ConstClass 를 참조하지 않고, 초기화도 안 일어난다.

### Test1 — 원본을 읽으러 감 (`javap -c -p Test1.class`)
```
3: getstatic     #13   // Field SubClass.value:I
```
`getstatic ...value` — 값을 복사한 게 아니라 *원본 필드를 실제로 읽으러* 간다. 그래서 그 필드를
선언한 SuperClass 가 초기화된다. (컴파일러는 접근 표기를 `SubClass.value` 로 기록하지만,
런타임에 깨어나는 건 필드를 실제 선언한 SuperClass 뿐 — 그래서 `SubClass init!` 은 안 찍힌다.)

## 배운 점 (이론 ↔ 실습 연결)

- **능동 참조 vs 수동 참조의 경계를 출력으로 봤다**: 예제 1은 능동 참조(static 필드 읽기)지만
  *부모만* 깨우고, 예제 2·3은 능동 참조처럼 보여도 초기화를 안 일으키는 수동 참조다.
- **복사 vs 원본 읽기가 바이트코드로 갈렸다**: `ldc`(값 복사, 원본 안 봄) vs `getstatic`(원본 읽기,
  초기화 유발). 노트 §3 보강의 "갈림점은 final 여부가 아니라 컴파일 때 값이 박히는가"를 실증.
- **static 필드는 선언한 클래스만 초기화**: `SubClass.value` 로 접근해도 `value` 의 주인은
  SuperClass 라 SuperClass 만 깨어났다.

## 비고
- `*.class` 는 컴파일 산출물이라 Drive 에 올리지 않는다(소스·기록만).
- 재현: `cd ~/jvm-practice/ch07-class-loading && javac *.java && java Test1/Test2/Test3`.
  바이트코드는 `javap -c -p Test3.class | grep -iE "ldc|getstatic"`.
