# packages·JAR·classpath 기초 실습

IDE와 Gradle 없이 JDK의 `javac`, `java`, `jar`, `javap`만 사용합니다. 라이브러리 클래스와 애플리케이션 클래스를 서로 다른 디렉터리에 컴파일하고, 라이브러리를 JAR로 바꾼 뒤, 마지막에는 실행 JAR까지 만듭니다.

이 실습은 `jvm-practice`의 Gradle 멀티모듈에 넣지 않았습니다. Gradle이 클래스패스를 자동으로 조립하면 관찰하려는 저수준 명령이 다시 가려지기 때문입니다.

## 구조

```text
classpath-basics/
├── README.md
├── scripts/
│   └── run.sh
└── src/
    ├── application/
    │   └── com/example/app/Main.java
    └── library/
        └── com/example/math/Calculator.java
```

`src/application`과 `src/library`는 서로 다른 소스 루트입니다. 컴파일 결과도 각각 `build/app-classes`와 `build/lib-classes`에 나뉩니다. 이 분리가 있어야 애플리케이션 클래스패스에서 라이브러리를 넣고 빼는 실험이 가능합니다.

## 전체 실행

```bash
cd ~/jvm-practice/classpath-basics
./scripts/run.sh all
```

전체 실행은 다음 순서로 진행합니다:

1. `Calculator.java`를 `build/lib-classes`에 컴파일합니다.
2. 해당 디렉터리를 컴파일 클래스패스에 놓고 `Main.java`를 컴파일합니다.
3. 두 클래스 디렉터리를 런타임 클래스패스에 놓고 실행합니다.
4. 계산기 클래스를 `lib/calculator.jar`로 묶습니다.
5. 디렉터리 대신 JAR를 컴파일·런타임 클래스패스에 놓고 실행합니다.
6. 두 클래스를 `build/calculator-app.jar`에 합치고 `java -jar`로 실행합니다.

출력에서 클래스가 실제로 읽힌 위치를 확인할 수 있습니다:

```text
20 + 22 = 42
Main.class       <- file:/.../build/app-classes/
Calculator.class <- file:/.../build/lib-classes/

20 + 22 = 42
Main.class       <- file:/.../build/app-classes/
Calculator.class <- file:/.../lib/calculator.jar
```

## 단계별 실행

```bash
./scripts/run.sh directory       # 두 class 디렉터리를 classpath로 실행
./scripts/run.sh library-jar     # 라이브러리 JAR를 classpath로 실행
./scripts/run.sh executable-jar  # Main-Class가 있는 실행 JAR 생성·실행
./scripts/run.sh inspect         # javap·JAR 엔트리·MANIFEST까지 확인
```

명령을 직접 입력하고 싶다면 `scripts/run.sh`의 같은 이름 함수를 순서대로 읽으면 됩니다. 각 함수는 학습 문서의 명령을 절대 경로로 안전하게 옮긴 형태입니다.

## 실패 재현

컴파일 클래스패스와 런타임 클래스패스는 별도 입력입니다. 다음 두 명령은 한쪽을 고의로 빠뜨려 서로 다른 실패 시점을 보여 줍니다:

```bash
./scripts/run.sh fail-compile  # javac가 Calculator를 찾지 못함
./scripts/run.sh fail-runtime  # JVM이 Calculator를 찾지 못함
```

`fail-compile`에서는 `package com.example.math does not exist`와 `cannot find symbol`이 나옵니다. `fail-runtime`에서는 컴파일은 성공하지만 실행 시 `NoClassDefFoundError`가 발생합니다. 두 스크립트는 해당 실패를 기대 결과로 처리하므로 마지막에는 종료 코드 0으로 끝납니다.

## 대응 학습 문서

`runners-high/write/01_language/java/06_Build/01-02.패키지·JAR·클래스패스.md`
