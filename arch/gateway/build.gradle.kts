plugins {
    java
    id("org.springframework.boot") version "4.1.1"
    id("io.spring.dependency-management") version "1.1.7"
}

group = "gatewaylab"
version = "0.0.1-SNAPSHOT"
description = "Learning lab: an HTTP reverse proxy / API gateway built on raw Netty, phase by phase"

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
    // 서블릿 컨테이너를 띄우지 않기 위해 web 스타터가 아니라 core 스타터만 쓴다.
    implementation("org.springframework.boot:spring-boot-starter")

    // Netty 버전은 Boot BOM 이 관리한다 (4.1.1 → netty 4.2.x).
    implementation("io.netty:netty-codec-http")
    implementation("io.netty:netty-handler")
    implementation("io.netty:netty-transport")

    testImplementation("org.springframework.boot:spring-boot-starter-test")
    testRuntimeOnly("org.junit.platform:junit-platform-launcher")
}

tasks.withType<Test> {
    useJUnitPlatform()
}
