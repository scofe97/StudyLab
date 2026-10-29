dependencies {
    implementation(project(":ch03-gc:common"))
}

application {
    mainClass.set("org.runners.jvm.ch03.g1.G1GcDemo")
    applicationDefaultJvmArgs = listOf(
        "-XX:+UseG1GC",
        "-Xms64m", "-Xmx64m",
        "-XX:MaxGCPauseMillis=100",
        "-Xlog:gc*=info:file=build/gc.log:time,uptime,level"
    )
}
