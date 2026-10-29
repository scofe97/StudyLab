# 1장 — 자바 기술 체계 실습

책 §1.5~§1.7 학습 노트(`../../ch01_java-tech/`)와 짝을 이루는 코드·스크립트를 모은다. 이 챕터는 OOM 재현 같은 코드 박제가 핵심이 아니라, "현재 돌고 있는 JVM이 무엇인지 시스템 속성으로 식별"하는 데모 한 편과 "OpenJDK를 직접 빌드"하는 절차의 박제가 본체다.

## 모듈 ↔ 책의 절 매핑

| 산출물 | 책 위치 | 다루는 것 | 실행 명령 |
|--------|--------|----------|-----------|
| `src/.../JavaTechSystemDemo.java` | §1.2 p.34 | 현재 JVM의 vendor·name·version·home을 시스템 속성으로 출력 | `./gradlew :ch01:run` |
| `scripts/install-build-deps-ubuntu.sh` | §1.6.3 p.55 | OpenJDK 빌드에 필요한 Ubuntu/Debian 패키지 목록 박제 | `./scripts/install-build-deps-ubuntu.sh` |
| `scripts/build-openjdk.sh` | §1.6.4 p.57 | OpenJDK 소스 clone → configure → make images 절차 박제 | `./scripts/build-openjdk.sh` |

데모 클래스는 실험용이 아니라 *책의 첫 페이지 인사*에 가깝다. JDK 21 toolchain이 실제로 어떤 vendor·VM 이름을 응답하는지 한 번 보고 넘어가는 용도다.

## 실행

### 데모 클래스

루트(`_practice/`)에서 Gradle task를 호출한다.

```bash
cd write/01_language/java/05_JVM/_practice

./gradlew :ch01:run
```

출력은 다음과 유사하다. 값은 로컬 toolchain 설치본에 따라 달라진다.

```console
java.version       = 21.0.3
java.vendor        = Eclipse Adoptium
java.vm.name       = OpenJDK 64-Bit Server VM
java.vm.version    = 21.0.3+9-LTS
java.home          = /Users/.../jdk-21.0.3.jdk/Contents/Home
```

### OpenJDK 빌드 스크립트

두 스크립트 모두 **dry-run이 기본**이다. 인자 없이 실행하면 무엇을 하려는지만 출력하고 종료하므로, 처음 한 번은 dry-run으로 계획을 확인하고 `-y`를 붙여 실제로 실행한다.

```bash
cd write/01_language/java/05_JVM/_practice/ch01/scripts

# 1) 빌드 도구 설치 — Ubuntu/Debian (sudo 필요)
./install-build-deps-ubuntu.sh           # 설치 대상 패키지 목록만 출력
./install-build-deps-ubuntu.sh -y        # 실제 apt-get install

# 2) OpenJDK 소스 빌드 — 부트 JDK 경로가 필요
./build-openjdk.sh                                   # 빌드 계획만 출력
./build-openjdk.sh -y --boot-jdk /path/to/jdk-20     # clone → configure → make images
```

빌드 결과는 `openjdk-src/build/<config>/images/jdk/bin/java` 에 떨어지며, 스크립트가 마지막에 `-version`을 호출해 빌드 성공을 검증한다.

## 주의 사항

`install-build-deps-ubuntu.sh`는 Ubuntu/Debian 전용이며 `apt-get`이 없는 환경에서는 즉시 종료한다. macOS에서는 책 §1.6.3 대신 Homebrew (`brew install autoconf ccache`)와 Xcode Command Line Tools가 같은 역할을 한다 — 자동화 스크립트는 별도로 제공하지 않는다.

`build-openjdk.sh`는 디스크 약 10GB와 30분 이상을 사용한다. 부트 JDK는 *빌드 대상보다 한 단계 낮은 LTS*를 권장한다(jdk21u 빌드 → JDK 20 부트). 부트 JDK 버전이 너무 낮거나 너무 높으면 `configure` 단계에서 실패한다.

`--with-debug-level=slowdebug` 옵션을 기본으로 둔 이유는 §1.7 정독 노트(`../../ch01_java-tech/02-03.실전 — OpenJDK 빌드하기.md`)에서 다룬다. 빌드 시간은 약 두 배 늘지만, inline이 풀린 디버거 친화적 바이너리를 얻는다.

## 관련 문서

- [`../../ch01_java-tech/`](../../ch01_java-tech/) — 1장 §1.5~§1.7 정독 노트 4편 (이 모듈의 이론적 근거)
- [`../README.md`](../README.md) — `_practice/` 전체 실행 가이드와 챕터 간 매핑
- [`ch02-memory-area/README.md`](../ch02-memory-area/README.md) — 다음 챕터, OOM 재현 모듈로 톤이 달라진다
