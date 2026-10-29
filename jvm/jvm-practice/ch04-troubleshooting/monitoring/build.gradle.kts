application {
    mainClass.set("org.runners.jvm.ch04.monitoring.ThreadMonitoringTest")
    applicationDefaultJvmArgs = listOf(
        // 책 §4.3.2 — JConsole 메모리 톱니 관찰용 고정 옵션
        "-Xms100m", "-Xmx100m",
        "-XX:+UseSerialGC",  // 영역 경계가 또렷해 학습 관찰에 유리
        // JConsole/JMC 가 JMX 로 붙도록 (로컬 attach 면 생략 가능)
        "-Dcom.sun.management.jmxremote"
    )
}
