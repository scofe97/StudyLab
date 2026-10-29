// 《Java Performance, 2nd Ed.》(Scott Oaks) Ch1 §4 — "그래, 어서, 섣부른 최적화도 하라"
// 로깅 가드(if log.isLoggable(...))가 "복잡도 ≈ 0, 성능 이득 큼"인지 JMH로 실측한다.
//
// JMH(jmh-core 1.37) + annotationProcessor 는 루트 build.gradle.kts 의 subprojects 블록에서
// 이미 상속되므로 여기서 재선언하지 않는다. application 플러그인도 루트에서 적용된다.
application {
    // main()에서 JMH Runner 를 직접 띄운다 → `./gradlew :jpf-logging-guard:run` 한 줄로 벤치 실행.
    mainClass.set("org.runners.jvm.jpf.logging.LoggingGuardBenchmark")
}
