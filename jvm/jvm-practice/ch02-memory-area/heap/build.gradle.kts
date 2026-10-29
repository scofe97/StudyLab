application {
    mainClass.set("org.runners.jvm.ch02.heap.HeapOOM")
    applicationDefaultJvmArgs = listOf(
        "-Xms20m",
        "-Xmx20m",
        "-XX:+HeapDumpOnOutOfMemoryError",
        "-XX:HeapDumpPath=build/heap.hprof"
    )
}
