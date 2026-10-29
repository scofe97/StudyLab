dependencies {
    implementation(project(":ch03-gc:common"))
}

application {
    mainClass.set("org.runners.jvm.ch03.shenandoah.ShenandoahDemo")
    applicationDefaultJvmArgs = listOf(
        "-XX:+UseShenandoahGC",
        "-Xms64m", "-Xmx64m",
        "-Xlog:gc*=info:file=build/gc.log:time,uptime,level"
    )
}
