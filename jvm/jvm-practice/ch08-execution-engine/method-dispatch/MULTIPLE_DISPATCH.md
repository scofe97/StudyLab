# 다중 디스패치 실습 — 오버로드 함정과 Visitor 이중 디스패치

> ch03 03-03 노트(다중 디스패치와 가상 메서드 테이블) §1-2와
> 03-02 노트 §실무의 실습. 자바가 동적 다중 디스패치를 지원하지 않아 생기는
> 함정을 `javap -c` 로 확인하고, 이중 디스패치(Visitor)로 우회한다.

## 관련 이론
- [03-03. 다중 디스패치와 가상 메서드 테이블](../../../03-03.다중 디스패치와 가상 메서드 테이블.md)
- [03-02. 메서드 호출 — 해석과 정적·동적 디스패치](../../../03-02.메서드 호출 — 해석과 정적·동적 디스패치.md)

## 실습 대상
- `MultipleDispatch.java` — 게임 예제. `Battle.attack(Character, Monster)` 오버로드 vs `attackedBy(this)` 이중 디스패치.
- `EventDispatch.java` — 실무 예제. `process(Event)` 오버로드 함정 vs 이벤트 객체 다형 위임.
- JDK: Temurin 21.

## ① 다중 디스패치 함정 — 오버로드는 선언 타입으로 컴파일 때 고정

`java MultipleDispatch` 출력:
```
오버로드: 기본 공격
이중 디스패치: 전사가 드래곤을 공격
```
실제 타입은 `Warrior`·`Dragon` 인데 오버로드 호출은 `기본 공격` 이 나온다. 선언 타입이 `Character`·`Monster` 라서다.

`javap -c -p MultipleDispatch` 의 main:
```
35: invokevirtual #30   // Method MultipleDispatch$Battle.attack:(LMultipleDispatch$Character;LMultipleDispatch$Monster;)V
48: invokevirtual #36   // Method MultipleDispatch$Character.attack:(LMultipleDispatch$Monster;)V
```
- 35번: 오버로드 호출의 시그니처가 `(Character;Monster;)V` 로 **선언 타입에 고정**. 실제 타입(Warrior/Dragon)을 안 본다.
- 48번: 이중 디스패치의 1차 호출 `Character.attack(Monster)`. 그 안에서 `monster.attackedBy(this)` 로 2차 디스패치가 일어나 `Dragon.attackedBy(Warrior)` 가 실행된다.

## ② 실무 함정 — 이벤트 핸들러 오버로드

`java EventDispatch` 출력:
```
오버로드 핸들러: 기본 처리
다형 위임: 사용자 생성 처리
```
`Event event = new UserCreatedEvent()` 인데 오버로드 핸들러는 `기본 처리` 가 불린다.

`javap -c -p 'EventDispatch$Dispatcher'` 의 handle:
```
2: invokevirtual #7   // Method process:(LEventDispatch$Event;)V
```
- `handle(Event)` 안의 `process(event)` 는 선언 타입 `Event` 만 보고 `process(Event)` 로 고정된다.
- `process(UserCreatedEvent)` 오버로드는 영영 안 불린다 = 오버로딩이 정적 디스패치라서다.
- 회피: 이벤트 객체에 `process()` 를 두고 위임하면 오버라이딩(동적)이라 실제 타입 버전이 불린다.

## 배운 점 (이론 ↔ 실습 연결)

- **오버로딩=정적 다중 디스패치**: 수신자+인자의 *선언 타입* 으로 컴파일 때 시그니처가 박힌다. 실제 타입 조합은 무시된다.
- **이중 디스패치로 우회**: 단일 디스패치(동적)를 두 번 겹치면 두 객체의 실제 타입 조합을 잡을 수 있다 — Visitor 패턴의 뼈대.
- **실무 함정**: 타입별 `process(XxxEvent)` 오버로드는 `process(Event)` 로 빨려 들어간다. 다형 위임·타입별 Handler 맵·sealed+switch 로 피한다.

## 비고
- `*.class` 는 컴파일 산출물이라 기록하지 않는다(소스·기록만).
- 재현: `cd ~/jvm-practice/ch08-execution-engine/method-dispatch && javac MultipleDispatch.java EventDispatch.java && java MultipleDispatch && java EventDispatch`
