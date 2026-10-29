dependencies {
    implementation(project(":ch03-gc:common"))
}

application {
    mainClass.set("org.runners.jvm.ch03.parallel.ParallelGcDemo")
    applicationDefaultJvmArgs = listOf(
        "-XX:+UseParallelGC",
        "-Xms64m", "-Xmx64m",
        "-XX:GCTimeRatio=99",
        "-Xlog:gc*=info:file=build/gc.log:time,uptime,level"
    )
}
