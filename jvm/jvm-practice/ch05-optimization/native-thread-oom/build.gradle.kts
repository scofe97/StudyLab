application {
    mainClass.set("org.runners.jvm.ch05.nativethread.NativeThreadOOM")
    applicationDefaultJvmArgs = listOf(
        // 책 §5.2 — "힙은 남는데 unable to create new native thread" 재현
        // 힙을 크게 잡아 같은 주소 공간에서 네이티브 스택 자리를 좁힌다(제로섬).
        "-Xmx256m"
    )
}
