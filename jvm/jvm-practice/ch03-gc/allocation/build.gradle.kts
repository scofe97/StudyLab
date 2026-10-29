dependencies {
    implementation(project(":ch03-gc:common"))
}

application {
    mainClass.set("org.runners.jvm.ch03.allocation.TestAllocation")
    applicationDefaultJvmArgs = listOf(
        // 책 §3.8.1 EdenAllocation 옵션
        "-Xms20m", "-Xmx20m", "-Xmn10m",
        "-XX:SurvivorRatio=8",
        "-XX:+UseParallelGC",  // 책 예제와 호환되는 GC 선택
        "-Xlog:gc*=info:file=build/gc.log:time,uptime,level"
    )
}

tasks.register<JavaExec>("runPretenureSize") {
    group = "application"
    description = "Run §3.8.2 PretenureSize demo with Serial GC"
    classpath = sourceSets["main"].runtimeClasspath
    mainClass.set("org.runners.jvm.ch03.allocation.PretenureSize")
    jvmArgs(
        "-Xms20m",
        "-Xmx20m",
        "-Xmn10m",
        "-XX:+UseSerialGC",
        "-XX:SurvivorRatio=8",
        "-XX:PretenureSizeThreshold=3145728",
        "-Xlog:gc*=info:file=build/pretenure-gc.log:time,uptime,level"
    )
}
