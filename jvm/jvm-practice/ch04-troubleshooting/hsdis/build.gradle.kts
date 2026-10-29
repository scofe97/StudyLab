application {
    mainClass.set("org.runners.jvm.ch04.hsdis.Bar")
    applicationDefaultJvmArgs = listOf(
        // 책 §4.4 — HSDIS 로 sum() 의 JIT 기계어를 디스어셈블해 출력
        // ⚠️ HSDIS 라이브러리(hsdis-amd64)가 JDK 경로에 있어야 어셈블리로 풀린다.
        //    없으면 16진 바이트만 나온다.
        "-XX:+UnlockDiagnosticVMOptions",
        "-XX:+PrintAssembly",
        "-Xcomp",  // 인터프리터 건너뛰고 즉시 컴파일해 기계어를 바로 보기
        "-XX:CompileCommand=compileonly,*Bar.sum",
        "-XX:CompileCommand=dontinline,*Bar.sum"
    )
}
