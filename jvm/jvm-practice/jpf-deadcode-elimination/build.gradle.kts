// 《Java Performance, 2nd Ed.》(Scott Oaks) Ch2 §2 — "결과를 반드시 읽어야 한다"
// 벤치마크가 계산 결과를 소비하지 않으면 JIT 가 그 계산을 통째로 제거(dead code elimination)해
// 측정값이 0(또는 빈 루프 시간)이 되는 함정을 JMH 로 실측한다.
//
// JMH(jmh-core 1.37) + annotationProcessor 는 루트 build.gradle.kts 의 subprojects 블록에서
// 이미 상속되므로 여기서 재선언하지 않는다. application 플러그인도 루트에서 적용된다.
application {
    // main()에서 JMH Runner 를 직접 띄운다 → `./gradlew :jpf-deadcode-elimination:run` 한 줄로 벤치 실행.
    mainClass.set("org.runners.jvm.jpf.deadcode.DeadCodeEliminationBenchmark")
}
