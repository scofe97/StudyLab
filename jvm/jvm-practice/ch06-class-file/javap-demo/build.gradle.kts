application {
    mainClass.set("org.runners.jvm.ch06.classfile.TestClass")
    // 실행 자체보다 *컴파일된 .class 를 javap 로 역어보는* 게 실습 목적이다.
    // compileJava 후 build/classes/java/main 에서 javap -v / javap -c 로 관찰한다.
}
