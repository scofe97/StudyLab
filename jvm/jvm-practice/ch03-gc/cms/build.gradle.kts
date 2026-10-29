dependencies {
    implementation(project(":ch03-gc:common"))
}

application {
    mainClass.set("org.runners.jvm.ch03.cms.CmsDemo")
    // CMS는 JDK 14에서 제거됨. JDK 21에서 옵션을 주면 경고 후 G1 으로 대체된다.
    // 박제 의의는 역사적 학습.
    applicationDefaultJvmArgs = listOf(
        "-XX:+UnlockDiagnosticVMOptions",
        "-Xms64m", "-Xmx64m",
        "-Xlog:gc*=info:file=build/gc.log:time,uptime,level"
        // -XX:+UseConcMarkSweepGC 를 의도적으로 빼고 코드 박제만. JDK 21에서 오류 종료 회피.
    )
}
