application {
    mainClass.set("org.runners.jvm.ch02.jvmstack.JavaVMStackSOF")
    applicationDefaultJvmArgs = listOf(
        "-Xss220k"
    )
}
