plugins {
    java
    id("org.springframework.boot") version "4.1.1" apply false
    id("io.spring.dependency-management") version "1.1.7" apply false
}

subprojects {
    apply(plugin = "java")
    apply(plugin = "org.springframework.boot")
    apply(plugin = "io.spring.dependency-management")

    group = "hublab"
    version = "0.0.1-SNAPSHOT"

    java {
        // 전역 ~/.gradle/gradle.properties 가 daemon JVM 을 17 로 고정하므로
        // 컴파일·실행 JDK 는 toolchain 으로 25 를 지정한다 (gateway-lab 과 같은 방식).
        toolchain {
            languageVersion = JavaLanguageVersion.of(25)
        }
    }

    repositories {
        mavenCentral()
    }

    dependencies {
        "testImplementation"("org.springframework.boot:spring-boot-starter-test")
        "testRuntimeOnly"("org.junit.platform:junit-platform-launcher")
    }

    tasks.withType<Test> {
        useJUnitPlatform()
    }
}
