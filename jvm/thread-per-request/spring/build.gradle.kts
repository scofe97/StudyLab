plugins {
    java
    id("org.springframework.boot") version "4.1.1"
    id("io.spring.dependency-management") version "1.1.7"
}

group = "tpr"
version = "0.0.1-SNAPSHOT"
description = "Learning lab: thread-per-request (platform vs virtual threads) compared with Go"

java {
    // 전역 ~/.gradle/gradle.properties 가 daemon JVM 을 17 로 고정하므로
    // 컴파일·실행 JDK 는 toolchain 으로 25 를 지정한다.
    toolchain {
        languageVersion = JavaLanguageVersion.of(25)
    }
}

repositories {
    mavenCentral()
}

dependencies {
    // Boot 4 에서 spring-boot-starter-web 은 spring-boot-starter-webmvc 로 바뀌었다 (Tomcat 포함).
    implementation("org.springframework.boot:spring-boot-starter-webmvc")

    testImplementation("org.springframework.boot:spring-boot-starter-test")
    testRuntimeOnly("org.junit.platform:junit-platform-launcher")
}

tasks.withType<Test> {
    useJUnitPlatform()
}
