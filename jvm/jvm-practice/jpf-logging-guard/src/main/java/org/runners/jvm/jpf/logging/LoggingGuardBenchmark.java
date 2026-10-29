package org.runners.jvm.jpf.logging;

import java.util.concurrent.TimeUnit;
import java.util.logging.Level;
import java.util.logging.Logger;

import org.openjdk.jmh.annotations.Benchmark;
import org.openjdk.jmh.annotations.BenchmarkMode;
import org.openjdk.jmh.annotations.CompilerControl;
import org.openjdk.jmh.annotations.Fork;
import org.openjdk.jmh.annotations.Measurement;
import org.openjdk.jmh.annotations.Mode;
import org.openjdk.jmh.annotations.OutputTimeUnit;
import org.openjdk.jmh.annotations.Scope;
import org.openjdk.jmh.annotations.Setup;
import org.openjdk.jmh.annotations.State;
import org.openjdk.jmh.annotations.Warmup;
import org.openjdk.jmh.infra.Blackhole;
import org.openjdk.jmh.runner.Runner;
import org.openjdk.jmh.runner.RunnerException;
import org.openjdk.jmh.runner.options.Options;
import org.openjdk.jmh.runner.options.OptionsBuilder;

/**
 * 《Java Performance, 2nd Ed.》(Scott Oaks) Ch1 §4 로깅 가드 실측.
 *
 * <p>저자의 주장: {@code if (log.isLoggable(...))} 가드는 "복잡도를 거의 올리지 않으면서"
 * 성능 이득을 주므로, Knuth 의 "섣부른 최적화" 격언이 가리키는 *미뤄야 할 최적화*가 아니라
 * 좋은 코더가 처음부터 하는 *기본 선택*이다. 이 벤치마크는 그 이득이 실제로 존재하는지,
 * 그리고 그 이득이 "문자열 연결 회피"뿐 아니라 "불필요한 메서드 호출 회피"에서도 오는지를
 * 숫자로 확인한다.
 *
 * <p>핵심 셋업: Logger 레벨을 {@link Level#INFO} 로 두어 {@link Level#FINE} 로그가
 * *출력되지 않는* 흔한 경우를 만든다. 이때 naive 버전은 출력도 안 될 메시지를 위해
 * 문자열 연결과 calcX()/calcY() 호출을 그대로 수행한다.
 */
@State(Scope.Thread)
@BenchmarkMode(Mode.AverageTime)        // 호출당 평균 소요 시간(ns/op)
@OutputTimeUnit(TimeUnit.NANOSECONDS)
@Warmup(iterations = 5, time = 1)        // JIT 워밍업 5회 → 정상 상태 측정
@Measurement(iterations = 5, time = 1)
@Fork(1)
public class LoggingGuardBenchmark {

    private Logger log;

    @Setup
    public void setUp() {
        log = Logger.getLogger(LoggingGuardBenchmark.class.getName());
        // FINE 메시지가 출력되지 않는 "흔한 경우". 운영 환경의 디버그 로그가 꺼진 상태와 같다.
        log.setLevel(Level.INFO);
    }

    /**
     * naive — 노트 §4 첫 번째 코드(72~74행) 재현.
     * 레벨이 INFO 라 FINE 메시지는 버려지지만, 인자 평가 규칙상
     * (1) "...X is " + calcX() + " and Y is " + calcY() 문자열 연결과
     * (2) calcX()/calcY() 호출이 log.log() 진입 *전에* 모두 일어난다.
     * 즉 출력되지도 않을 메시지를 만드느라 일을 다 해버린다.
     */
    @Benchmark
    public void naiveLogging(Blackhole bh) {
        log.log(Level.FINE, "I am here, and the value of X is "
                + calcX() + " and Y is " + calcY());
        // log.log() 가 내부적으로 레벨 체크 후 메시지를 버려도, 위 연결·호출 비용은 이미 발생함.
        bh.consume(log);
    }

    /**
     * guarded — 노트 §4 두 번째 코드(79~83행) 재현.
     * isLoggable() 가드가 false 면 그 안의 문자열 연결·calcX()/calcY() 호출·
     * Object[] 배열 할당이 *통째로* 스킵된다. 추가된 복잡도는 if 한 줄뿐.
     */
    @Benchmark
    public void guardedLogging(Blackhole bh) {
        if (log.isLoggable(Level.FINE)) {
            log.log(Level.FINE,
                    "I am here, and the value of X is {0} and Y is {1}",
                    new Object[] {calcX(), calcY()});
        }
        bh.consume(log);
    }

    /**
     * 인라이닝을 막아 "불필요한 메서드 호출"의 비용을 가시화한다.
     * 실제 코드라면 DB 조회·직렬화처럼 더 비싼 계산일 수 있다 — 여기선 가벼운 연산으로 대표.
     */
    @CompilerControl(CompilerControl.Mode.DONT_INLINE)
    private int calcX() {
        int acc = 0;
        for (int i = 0; i < 64; i++) {
            acc += (i * 31) ^ acc;
        }
        return acc;
    }

    @CompilerControl(CompilerControl.Mode.DONT_INLINE)
    private int calcY() {
        int acc = 1;
        for (int i = 0; i < 64; i++) {
            acc += (i * 17) ^ acc;
        }
        return acc;
    }

    public static void main(String[] args) throws RunnerException {
        Options opt = new OptionsBuilder()
                .include(LoggingGuardBenchmark.class.getSimpleName())
                .build();
        new Runner(opt).run();
    }
}
