dependencies {
    implementation(project(":ch03-gc:common"))
}

application {
    mainClass.set("org.runners.jvm.ch03.serial.SerialGcDemo")
    applicationDefaultJvmArgs = listOf(
        "-XX:+UseSerialGC",
        "-Xms64m", "-Xmx64m",
        "-Xlog:gc*=info:file=build/gc.log:time,uptime,level"
    )
}
