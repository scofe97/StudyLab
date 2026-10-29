# 필드 가려짐 vs 메서드 오버라이딩 실습 — 메서드는 동적, 필드는 정적

> ch03 03-03 노트(다중 디스패치와 vtable)의 Phase 3 실습.
> `FieldHasNoPolymorphic` 한 예제로 "메서드는 실제 타입(동적) / 필드는 선언 타입(정적)" 비대칭과,
> 그 위에 겹치는 "부모→자식 생성자 순서" 함정을 함께 확인한다.

## 관련 이론
- [03-03. 다중 디스패치와 가상 메서드 테이블](../../../03-03.다중 디스패치와 가상 메서드 테이블.md) §3

## 실습 대상
- `FieldHasNoPolymorphic.java` — Father/Son 에 같은 이름 `money` 필드 + `showMeMoney()` 오버라이딩.
- JDK: Temurin 21.

## 실행 결과

`java FieldHasNoPolymorphic`:
```
I am Son, I have $0
I am Son, I have $4
This guy has $2
```

## 세 줄 해석

| 출력 | 이유 |
|------|------|
| `I am Son, $0` | 메서드=동적(Son 버전 실행) + 그 시점 Son.money 가 아직 초기화 전이라 기본값 0 |
| `I am Son, $4` | Son 생성자에서 다시 호출, 이번엔 money=4 채워진 뒤 |
| `This guy $2` | 필드=정적, `guy` 의 선언 타입 Father → Father.money(=2) |

### ① 메서드는 동적 — 두 줄 다 "I am Son"
`Father guy = new Son()` 에서 Father 생성자가 부른 `showMeMoney()` 도 실제 타입 Son 의 것이 실행된다.
`invokevirtual` 이 수신자의 실제 타입(Son)을 따라가기 때문. "I am Father" 는 한 번도 안 나온다.

### ② 첫 줄이 $0 인 이유 — 생성자 순서
객체 생성은 *부모 생성자 → 자식 생성자* 순서다.
```
Father() 실행 → showMeMoney() (동적) → Son.money 읽음  ← 아직 Son 생성자 전!
Son() 실행 → money = 3 → money = 4                      ← 여기서야 Son.money 채워짐
```
Father 생성자가 Son.money 를 읽는 시점에 Son 생성자가 아직 안 돌아, JVM 이 준비 단계에 채운 *기본값 0* 이 보인다.
→ 이것이 "생성자에서 오버라이드 가능한 메서드를 호출하지 마라"(Effective Java) 의 근거.

### ③ 필드는 정적 — guy.money = 2
`guy.money` 는 필드라 `guy` 의 *선언 타입* Father 를 따른다. 같은 Son 객체인데도 Father.money(2) 가 보인다.
같은 이름 필드는 오버라이딩이 아니라 *가려짐(field hiding)*.

## javap 로 확인
`javap -c FieldHasNoPolymorphic`:
- `showMeMoney()` 호출 → `invokevirtual` (동적 디스패치)
- `guy.money` 접근 → `getfield #N // Field ...$Father.money:I` — *선언 타입 Father* 로 박힘(정적)

## 배운 점 (이론 ↔ 실습 연결)
- **메서드는 실제 타입, 필드는 선언 타입** — 같은 객체라도 비대칭. javap 의 invokevirtual vs getfield(Father) 로 실증.
- **생성자 순서 함정** — 부모 생성자가 자식의 미초기화 필드를 동적 메서드로 읽으면 기본값(0)이 보인다.

## 비고
- `*.class` 는 컴파일 산출물이라 Drive 에 올리지 않는다(소스·기록만).
- 재현: `cd ~/jvm-practice/ch08-execution-engine/method-dispatch && javac FieldHasNoPolymorphic.java && java FieldHasNoPolymorphic && javap -c FieldHasNoPolymorphic`
