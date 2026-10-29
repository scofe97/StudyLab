# 검증·준비 실습 — .class 안의 ConstantValue·StackMapTable 직접 보기

> ch03 02-02 노트(로딩·검증·준비)의 Phase 3 실습. 본문이 "컴파일러가 붙인다"고 설명한 두 속성
> (`ConstantValue`, `StackMapTable`)이 실제 `.class` 안에 들어 있는 것을 `javap`로 눈으로 확인한다.

## 관련 이론
- [02-02. 로딩·검증·준비](../../../ch03_class-loading-mechanism/02-02.로딩·검증·준비.md) §2~3

## 실습 대상
- `PrepareDemo.java` — 일반 `static int value = 123` vs `static final int CONST = 456` vs `static final Object`(런타임 상수).
- `VerifyDemo.java` — 분기·반복 있는 `branchy()` vs 일직선 `noBranch()`.
- JDK: Temurin 21.0.3.
- 실행: `javac *.java && javap -v -p <Class>.class`

## ① 준비 단계 — ConstantValue 는 상수에만 붙는다

`javap -v -p PrepareDemo.class` 실측:
```
public static final int CONST;
    flags: (0x0019) ACC_PUBLIC, ACC_STATIC, ACC_FINAL
    ConstantValue: int 456          ← 상수에만 ConstantValue 속성

static {};                          ← <clinit>
    2: putstatic  #13  // Field value:I   ← value=123 대입은 여기 모임
```
- `CONST`(static final)에는 **`ConstantValue: int 456`** 속성이 붙는다 → 준비 단계에서 즉시 456.
- `value`(일반 static)에는 ConstantValue 가 **없다.** 대입 `putstatic value` 는 `<clinit>`에 모여
  *초기화* 때 실행 → 준비 직후엔 0.
- 본문 §3의 "일반 static은 0, ConstantValue 상수만 즉시 실제 값"이 그대로 보인다.
- 참고: `static final Object RUNTIME_CONST = new Object()` 처럼 런타임에 값이 정해지는 final 에는
  ConstantValue 가 안 붙는다(컴파일 때 값을 못 박으니 복사 불가).

## ② 바이트코드 검증 — StackMapTable 의 "분기점"

`javap -v -p VerifyDemo.class` 실측:
```
branchy(int): StackMapTable: number_of_entries = 4
    frame_type = 253 /* append */   ← 루프 변수 i 가 등장하는 지점
    frame_type =  17 /* same */
    frame_type =   3 /* same */
    frame_type = 250 /* chop */     ← i 가 스코프에서 사라지는 지점
noBranch(int,int): StackMapTable 없음   ← 일직선이라 합류 지점이 없음
```

### "분기점"의 정체 — 함수 호출이 아니라 *점프 명령의 목적지*

StackMapTable 의 frame 은 *함수 호출* 자리에 찍히는 게 아니다. **실행 경로가 갈라지거나 합류하는
지점 = 점프 명령(if/goto)의 목적지**에 찍힌다. `branchy()`의 바이트코드를 보면:
```
6:  if_icmpge 32   ← for 조건 거짓이면 32번지로 점프(루프 탈출) = 분기점
19: goto      26   ← 루프 맨 위(26번지)로 되돌아옴(백워드 점프) = 분기점
```
이 점프 *목적지*(32, 26 ...)가 곧 frame offset 이다.

왜 거기에 찍나: 일직선 코드는 위에서부터 따라가면 타입이 저절로 정해져 frame 이 필요 없다. 그러나
*여러 경로가 합류하는 지점*(if-else 끝, 루프 맨 위)은 어느 경로로 왔느냐에 따라 스택 상태가 다를 수
있어, "여기서는 타입이 이래야 한다"고 못박아 줘야 검증기가 헷갈리지 않는다. 그래서 `for`+`if/else`가
있는 `branchy`는 frame 4개, `return a+b` 한 줄인 `noBranch`는 0개다.

자바 코드 ↔ 분기점 대응:
| 자바 | 바이트코드 분기점 |
|------|-------------------|
| `if/else` | 조건 거짓일 때 건너뛸 목적지(else 시작) |
| `for/while` | 루프 맨 위로 되돌아오는 목적지 + 탈출 지점 |
| `switch` | 각 case 라벨 |
| `try/catch` | 예외 핸들러 시작 |
| 함수 호출 | (분기점 아님 — 다녀와서 다음 줄로 이어짐) |

## 배운 점 (이론 ↔ 실습 연결)

- **본문의 두 속성을 실물로 확인**: ConstantValue·StackMapTable 이 "컴파일러가 붙인다"던 그대로
  `.class` 안에 있었다.
- **준비=0 함정의 증거**: value 엔 ConstantValue 가 없고 putstatic 이 `<clinit>`에 모여, 준비 때 0
  임을 바이트코드로 확인.
- **검증 가속의 원리를 frame 으로**: branchy 4개 / noBranch 0개 — 분기가 많을수록 frame 이 늘고,
  검증기는 이 표와 *대조*만 하면 된다. 분기점은 함수 호출이 아니라 점프 목적지다.

## 비고
- `*.class` 는 컴파일 산출물이라 Drive 에 올리지 않는다(소스·기록만).
- 재현: `cd ~/jvm-practice/ch07-class-loading/verify-prepare && javac *.java && javap -v -p PrepareDemo.class` / `VerifyDemo.class`.
