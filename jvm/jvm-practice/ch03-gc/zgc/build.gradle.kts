dependencies {
    implementation(project(":ch03-gc:common"))
}

application {
    mainClass.set("org.runners.jvm.ch03.zgc.ZgcDemo")
    applicationDefaultJvmArgs = listOf(
        "-XX:+UseZGC",
        "-XX:+ZGenerational",  // JDK 21 Generational ZGC
        "-Xms256m", "-Xmx256m",  // ZGC는 작은 힙(<128MB)에서 동작 안 함
        "-Xlog:gc*=info:file=build/gc.log:time,uptime,level"
    )
}
