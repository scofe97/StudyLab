rootProject.name = "jvm-deep-dive"

include(":ch01")

// 2장 자바 메모리 영역과 메모리 오버플로
include(":ch02-memory-area:heap")
include(":ch02-memory-area:jvm-stack")
include(":ch02-memory-area:native-stack")
include(":ch02-memory-area:method-area")
include(":ch02-memory-area:constant-pool")
include(":ch02-memory-area:direct-memory")
include(":ch02-memory-area:layout")

// 3장 가비지 컬렉터와 메모리 할당 전략
include(":ch03-gc:common")
include(":ch03-gc:serial")
include(":ch03-gc:parallel")
include(":ch03-gc:cms")
include(":ch03-gc:g1")
include(":ch03-gc:zgc")
include(":ch03-gc:shenandoah")
include(":ch03-gc:allocation")

// 4장 가상 머신 성능 모니터링과 문제 해결 도구
include(":ch04-troubleshooting:monitoring")
include(":ch04-troubleshooting:btrace-target")
include(":ch04-troubleshooting:hsdis")

// 5장 최적화 사례 분석 및 실전
include(":ch05-optimization:native-thread-oom")

// 6장 클래스 파일 구조
include(":ch06-class-file:javap-demo")

// 9장 클래스 로딩 메커니즘 — 톰캣 클래스 로더 실습(04-01 노트 대응)
// 격리/클래스패스/TCCL/ClassLoader Leak 을 URLClassLoader 로 직접 재현
include(":ch03-classloader")

// SPI 직접 작성 실습 — ServiceLoader + META-INF/services 손으로 등록
include(":spi-demo")

// 《Java Performance》(Scott Oaks) Ch1 §4 — 로깅 가드(섣부른 최적화 아님) 실측
include(":jpf-logging-guard")

// 《Java Performance》(Scott Oaks) Ch2 §2 — dead code 제거(결과 미소비 시 JIT가 계산 삭제) 실측
include(":jpf-deadcode-elimination")
