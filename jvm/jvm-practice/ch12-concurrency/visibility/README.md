# ch12 · 가시성 버그 실습 — volatile 없는 공유 flag

**대응 노트**: `05_JVM/ch05_efficient-concurrency/01-01.하드웨어 효율과 자바 메모리 모델.md` §3·§5
**책**: 《JVM 밑바닥까지 파헤치기》 §12 효율적 동시성

## 무엇을 보는가

한 스레드가 공유 `stop` 플래그를 `true`로 바꿔 다른 스레드의 루프를 멈추려 한다.
`volatile`이 없으면 **루프가 영영 안 멈출 수 있다** — JMM의 메인/작업 메모리 분리가 만든 가시성 버그다.

| | A 쪽 구멍 | B 쪽 구멍 |
|---|---|---|
| 증상 | 바꾼 값이 메인 메모리에 안 내려감 | 메인 메모리를 다시 안 읽고 옛 복사본 재사용 |
| 8연산 | `store`·`write` 누락 | `read`·`load` 갱신 안 함 |
| 거드는 것 | — | JIT가 `while(!stop)` → `while(true)` hoist |

`volatile`은 두 구멍을 **동시에** 막는다(쓰기 즉시 반영 + 읽기 항상 새로).

## 실행

```bash
cd ~/jvm-practice/ch12-concurrency/visibility

# 1) 버그 재현 — 안 멈추고 매달릴 수 있다
javac VisibilityBug.java
java -XX:-TieredCompilation VisibilityBug     # 서버 JIT 강제 시 재현율 ↑

# 2) 수정본 — 1초 뒤 반드시 멈춘다
javac VisibilityFixed.java
java VisibilityFixed
```

## 관찰 포인트

- 두 파일의 차이는 **`volatile` 한 단어뿐**이다(`static volatile boolean stop`).
- 버그 버전은 환경(OS·CPU·JIT 모드)에 따라 우연히 멈출 수도 있다 — 그게 더 위험하다.
  "내 PC에선 되던데"가 운영에서 터지는 전형적 동시성 버그의 얼굴이다.
- `-XX:-TieredCompilation`으로 C2(서버) 컴파일러를 바로 태우면 루프 hoist가 잘 일어나 재현율이 올라간다.
