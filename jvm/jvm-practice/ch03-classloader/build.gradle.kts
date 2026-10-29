// ch03-classloader — 톰캣 클래스 로더 실습 (04-01 노트 대응)
//
// 이 모듈은 보통 모듈과 다르다. 실험들이 "같은 FQCN 의 다른 버전"과
// "앱 클래스패스에 없는 구현체"를 URLClassLoader 로 따로 로딩해야 한다.
// 그래서 libsrc/ 의 코드를 main 과 *분리된 별도 디렉터리*(build/labout/<name>)로
// 컴파일하고, 실험 실행 시 -Dlab.out 으로 그 위치를 알려준다.
//
// 루트 build.gradle.kts 가 java/application/toolchain(21) 을 이미 적용한다.

val labOut = layout.buildDirectory.dir("labout")
val mainOut = tasks.named<JavaCompile>("compileJava").flatMap { it.destinationDirectory }

// 커스텀 JavaCompile 태스크도 toolchain(21) 의 javac 를 쓰게 한다.
// (source/targetCompatibility 문자열만 주면 런처 JVM 의 javac 를 쓰려다 실패)
val java21 = javaToolchains.compilerFor {
    languageVersion.set(JavaLanguageVersion.of(21))
}

// 각 lab 산출물을 JavaCompile 태스크로 컴파일(Gradle 9 호환 — exec 미사용).
// v1/v2: 같은 FQCN 을 따로. plugin/provider: main 의 Plugin/Codec 에 의존.
fun registerLabCompile(name: String, needsMain: Boolean): TaskProvider<JavaCompile> =
    tasks.register<JavaCompile>("compileLab${name.replaceFirstChar { it.uppercase() }}") {
        source = fileTree("libsrc/$name") { include("**/*.java") }
        if (needsMain) {
            dependsOn(tasks.named("compileJava"))
            classpath = files(mainOut)
        } else {
            classpath = files()
        }
        destinationDirectory.set(labOut.map { it.dir(name) })
        options.encoding = "UTF-8"
        javaCompiler.set(java21)
    }

val labV1 = registerLabCompile("v1", needsMain = false)
val labV2 = registerLabCompile("v2", needsMain = false)
val labPlugin = registerLabCompile("plugin", needsMain = true)
val labProvider = registerLabCompile("provider", needsMain = true)

// provider 의 META-INF/services 리소스와 app(인터페이스) 디렉터리 구성
val labResources by tasks.registering(Copy::class) {
    dependsOn(labProvider, tasks.named("compileJava"))
    // ServiceLoader 가 읽는 services 파일
    from("libsrc/provider/META-INF") { into("provider/META-INF") }
    // "app" = Plugin/Codec 인터페이스(=main 출력의 일부)
    from(mainOut) {
        into("app")
        include("com/acme/api/**", "com/acme/spi/**")
    }
    into(labOut)
}

val compileLab by tasks.registering {
    description = "lab 산출물(v1/v2/plugin/provider/app) 전체 구성"
    dependsOn(labV1, labV2, labPlugin, labProvider, labResources)
}

// 실험별 실행 태스크: -Dlab.out 으로 별도 산출물 위치를 주입
fun registerExp(
    taskName: String,
    main: String,
    extraArgs: List<String> = emptyList(),
    jvm: List<String> = emptyList(),
    tolerateNonZeroExit: Boolean = false,
) {
    tasks.register<JavaExec>(taskName) {
        group = "lab"
        description = "$taskName 실행"
        dependsOn(compileLab)
        classpath = sourceSets["main"].runtimeClasspath
        mainClass.set(main)
        systemProperty("lab.out", labOut.get().asFile.absolutePath)
        jvmArgs(jvm)
        args(extraArgs)
        // exp4Leak 은 OutOfMemoryError 로 끝나는 게 *기대 결과*다.
        // 비정상 종료를 BUILD FAILED 로 보지 않게 한다.
        isIgnoreExitValue = tolerateNonZeroExit
    }
}

registerExp("exp1", "com.acme.lab.Exp1_TypeIdentity")
registerExp("exp2", "com.acme.lab.Exp2_ClasspathIsolation")
registerExp("exp3", "com.acme.lab.Exp3_TcclSpi")
registerExp("exp4Leak", "com.acme.lab.Exp4_ClassLoaderLeak", listOf("leak"), listOf("-XX:MaxMetaspaceSize=32m"), tolerateNonZeroExit = true)
registerExp("exp4NoLeak", "com.acme.lab.Exp4_ClassLoaderLeak", listOf("noleak"), listOf("-XX:MaxMetaspaceSize=32m"))

// application 플러그인 기본 run = 실험 1 (IntelliJ 기본 실행 대비)
application {
    mainClass.set("com.acme.lab.Exp1_TypeIdentity")
}
tasks.named<JavaExec>("run") {
    dependsOn(compileLab)
    systemProperty("lab.out", labOut.get().asFile.absolutePath)
}
