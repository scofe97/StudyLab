# 클래스 로더 실습 — 동일성(이름 + 로더) 직접 보기

> ch03 02-04 노트(클래스 로더와 부모 위임)의 Phase 3 실습. ①3계층 로더를 getClassLoader()로
> 확인하고, ②같은 .class 를 다른 로더로 로딩하면 instanceof 가 false 가 되는 것을 재현한다.

## 관련 이론
- [02-04. 클래스 로더와 부모 위임 모델](../../../ch03_class-loading-mechanism/02-04.클래스 로더와 부모 위임 모델.md) §1~2

## 실습 대상
- `LoaderInspect.java` — String/ArrayList/내 클래스의 로더 + 부모 체인 출력.
- `ClassLoaderIdentity.java` — 사용자 정의 로더로 자신을 다시 로딩 → instanceof 비교.
- JDK: Temurin 21.0.3.

## ① 3계층 로더 확인

```
$ java LoaderInspect
String 로더      = null
ArrayList 로더   = null
내 클래스 로더    = jdk.internal.loader.ClassLoaders$AppClassLoader@63c12fb0

-- 로더 부모 체인 (자식 → 부모) --
  jdk.internal.loader.ClassLoaders$AppClassLoader@...
  jdk.internal.loader.ClassLoaders$PlatformClassLoader@...
  null  ← 부트스트랩 (C++ 구현이라 자바 객체 없음)
```
- `String`·`ArrayList`(java.*) → **null** = 부트스트랩이 로딩(자바 객체가 아니라 null).
- 내 클래스 → **AppClassLoader** = 애플리케이션 로더.
- 부모 체인 `App → Platform → null(부트스트랩)` 이 위임 방향(자식이 부모를 필드로 들고 있음)을 보여준다.

### 디테일 — "확장 로더"가 아니라 PlatformClassLoader
노트는 3계층을 부트스트랩·*확장*·애플리케이션으로 설명하지만, 실측 부모 체인의 중간은
`PlatformClassLoader` 다. JDK 9 에서 확장 클래스 로더가 플랫폼 클래스 로더로 바뀌었기 때문이다
(모듈 시스템 도입). 이 변화가 바로 다음 글(02-05)의 주제 — 실습으로 미리 만난 셈이다.

## ② 같은 .class, 다른 로더 → instanceof false

```
$ java ClassLoaderIdentity
obj.getClass()                     = class ClassLoaderIdentity
obj instanceof ClassLoaderIdentity = false
```
- `obj.getClass()` 의 이름은 `ClassLoaderIdentity` 로 **같다.**
- 그런데 `instanceof` 는 **false.** `obj` 는 사용자 정의 로더가 로딩한 것이고, `instanceof` 오른쪽은
  App 로더가 로딩한 것이라 — 같은 바이트인데도 JVM 에겐 *별개의 클래스*다.
- 클래스 동일성 = *이름 + 로더* 의 쌍임이 직접 드러난다.

## 배운 점 (이론 ↔ 실습 연결)

- **로더가 곧 네임스페이스**: 같은 이름·같은 바이트라도 로더가 다르면 별개 클래스(instanceof false).
  이 성질이 톰캣 웹앱 격리·핫 디플로이의 기반.
- **부트스트랩은 null**: java.* 의 로더가 null 인 것으로 "C++ 구현이라 자바 객체가 없다"를 확인.
- **JDK 9 변화를 실습으로 선취**: 부모 체인의 PlatformClassLoader 가 02-05(모듈 시스템)의 예고.

## 비고
- `*.class` 는 컴파일 산출물이라 Drive 에 올리지 않는다(소스·기록만).
- 재현: `cd ~/jvm-practice/ch07-class-loading/classloader && javac *.java && java LoaderInspect` / `java ClassLoaderIdentity`.
