dependencies {
    implementation("net.bytebuddy:byte-buddy:1.14.18")
}

application {
    mainClass.set("org.runners.jvm.ch02.methodarea.MethodAreaOOM")
    applicationDefaultJvmArgs = listOf(
        "-XX:MetaspaceSize=10m",
        "-XX:MaxMetaspaceSize=10m"
    )
}
