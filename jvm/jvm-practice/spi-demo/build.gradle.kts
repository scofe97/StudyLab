// spi-demo — 순수 자바 SPI(ServiceLoader + META-INF/services) 직접 작성 실습.
//
// ch03-classloader 와 달리 표준 레이아웃이다. META-INF/services 등록 파일을
// src/main/resources 에 두면 Gradle 이 알아서 클래스패스에 포함한다.
//
// 실행:
//   ./gradlew :spi-demo:run --args="toss 10000"
//   ./gradlew :spi-demo:run --args="kakao 5000"
//   ./gradlew :spi-demo:run --args="naver 1000"   # 미등록 → 못 찾음

application {
    mainClass.set("com.myapp.SpiDemo")
}
