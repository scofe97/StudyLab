application {
    mainClass.set("org.runners.jvm.ch04.btrace.BTraceTarget")
    // BTrace 가 동적으로 붙을 대상이므로 특별한 JVM 옵션은 필요 없다.
    // 관찰: 이 프로세스를 띄운 뒤 VisualVM BTrace 플러그인으로 attach 해
    //       add(int,int) 메서드의 인자/반환을 재시작 없이 추적한다.
}
