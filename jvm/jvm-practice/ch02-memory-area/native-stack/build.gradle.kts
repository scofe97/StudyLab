application {
    mainClass.set("org.runners.jvm.ch02.nativestack.JavaVMStackOOM")
    applicationDefaultJvmArgs = listOf(
        "-Xss2m"
    )
}
