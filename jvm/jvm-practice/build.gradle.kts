plugins {
    java
}

subprojects {
    apply(plugin = "java")
    apply(plugin = "application")

    group = "org.runners.jvm"
    version = "1.0.0-SNAPSHOT"

    repositories {
        mavenCentral()
    }

    java {
        toolchain {
            languageVersion.set(JavaLanguageVersion.of(21))
        }
    }

    dependencies {
        "implementation"("org.openjdk.jol:jol-core:0.17")
        "implementation"("org.ow2.asm:asm:9.7")
        "implementation"("org.ow2.asm:asm-tree:9.7")
        "implementation"("org.ow2.asm:asm-util:9.7")
        "implementation"("org.openjdk.jmh:jmh-core:1.37")
        "annotationProcessor"("org.openjdk.jmh:jmh-generator-annprocess:1.37")

        "testImplementation"("org.junit.jupiter:junit-jupiter:5.10.2")
        "testImplementation"("org.assertj:assertj-core:3.25.3")
    }

    tasks.withType<JavaCompile> {
        options.encoding = "UTF-8"
        options.compilerArgs.add("-Xlint:all")
    }

    tasks.withType<Test> {
        useJUnitPlatform()
        testLogging {
            events("passed", "skipped", "failed")
        }
    }
}
