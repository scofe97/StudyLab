application {
    mainClass.set("org.runners.jvm.ch02.constantpool.RuntimeConstantPoolOOM")
    applicationDefaultJvmArgs = listOf(
        "-Xms10m",
        "-Xmx10m"
    )
}
