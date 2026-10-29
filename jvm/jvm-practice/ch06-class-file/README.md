# 6장 — 클래스 파일 구조 실습

책 §6.3의 클래스 파일 구조와 §6.4의 바이트코드를 *실행*이 아니라 *`javap`로 역어보며* 익힌다. 6장 실습의 핵심은 컴파일된 `.class`를 디스어셈블해 magic·상수 풀·접근 플래그·바이트코드를 눈으로 확인하는 것이다.

## 모듈 ↔ 책의 절 매핑

| 모듈 | 책 위치 | 관찰 방법 | 비고 |
|------|--------|-----------|------|
| `javap-demo` | §6.3~§6.4 | `javap -v` / `javap -c` | `TestClass`(필드·메서드 1개씩)를 디스어셈블. `main()` 없으니 `run` 아닌 `compileJava` |

## 실행 — compile 후 javap

```bash
# 1) 컴파일만 (TestClass 는 main 이 없어 run 하지 않는다)
./gradlew :ch06-class-file:javap-demo:compileJava

# 2) javap 로 클래스 파일 전체 구조 역어보기
CLS=ch06-class-file/javap-demo/build/classes/java/main/org/runners/jvm/ch06/classfile/TestClass.class

# magic·버전·상수 풀·access_flags·필드·메서드·속성 전체
javap -v "$CLS"

# inc()/getM() 의 바이트코드(iload·getfield·getstatic·iadd·ireturn)
javap -c "$CLS"
```

`javap -v` 출력에서 노트(`../../ch06_class-file/02-01.클래스 파일 구조.md`)의 ClassFile 구조 — magic `cafebabe`, Constant pool, `flags: ACC_PUBLIC`, 필드/메서드 — 가 그대로 보인다. `javap -c`는 02-02 노트의 옵코드(`getfield`·`getstatic`·`iadd`)를 실제로 확인하는 짝이다.

## 노트 연결

- 본편: [`../../ch06_class-file/02-01.클래스 파일 구조.md`](../../ch06_class-file/02-01.클래스%20파일%20구조.md), [`../../ch06_class-file/02-02.바이트코드 명령어.md`](../../ch06_class-file/02-02.바이트코드%20명령어.md)
