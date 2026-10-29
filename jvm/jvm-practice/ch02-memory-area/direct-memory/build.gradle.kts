application {
    mainClass.set("org.runners.jvm.ch02.directmemory.DirectMemoryOOM")
    applicationDefaultJvmArgs = listOf(
        "-Xmx20m",
        "-XX:MaxDirectMemorySize=10m",
        // Unsafe 접근 허용
        "--add-opens=java.base/jdk.internal.misc=ALL-UNNAMED"
    )
}
