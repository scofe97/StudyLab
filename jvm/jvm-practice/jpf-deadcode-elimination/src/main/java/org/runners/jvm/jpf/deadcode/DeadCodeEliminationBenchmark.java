package org.runners.jvm.jpf.deadcode;

import java.util.concurrent.TimeUnit;

import org.openjdk.jmh.annotations.Benchmark;
import org.openjdk.jmh.annotations.BenchmarkMode;
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
 * 《Java Performance, 2nd Ed.》(Scott Oaks) Ch2 §2 첫 번째 함정 실측.
 *
 * <p>저자의 주장: 마이크로벤치마크가 계산 결과를 *어디서도 읽지 않으면*, JIT 컴파일러는
 * 그 계산을 통째로 제거(dead code elimination)해도 된다고 판단한다. 그러면 측정값이
 * 0(또는 빈 루프 시간)이 나와, "이 연산이 빠르다"는 완전히 틀린 결론을 준다.
 * 해법은 결과를 반드시 *소비*하는 것이고, 노트는 {@code volatile} 인스턴스 변수를 권한다.
 *
 * <p>이 벤치마크는 같은 계산({@link #fib(int)})을 세 가지 방식으로 소비해 그 차이를 숫자로 본다.
 * <ul>
 *   <li>{@link #deadCodeNaive()} — 결과를 지역 변수에만 담고 버린다. JIT 가 계산을 제거할 수 있다.</li>
 *   <li>{@link #consumedByBlackhole(Blackhole)} — JMH 의 Blackhole 로 소비. 계산이 살아남는다.</li>
 *   <li>{@link #consumedByVolatile()} — volatile 필드에 써서 소비. 노트가 말한 그 해법.</li>
 * </ul>
 *
 * <p>기대: naive 는 사실상 0에 가깝게(계산이 사라져) 나오고, Blackhole·volatile 은
 * 실제 fib 계산 비용을 드러낸다. 이 차이가 "결과를 안 읽으면 측정이 거짓이 된다"의 증거다.
 */
@State(Scope.Thread)
@BenchmarkMode(Mode.AverageTime)        // 호출당 평균 소요 시간(ns/op)
@OutputTimeUnit(TimeUnit.NANOSECONDS)
@Warmup(iterations = 5, time = 1)        // JIT 워밍업 5회 → dead code 제거가 일어날 시간을 충분히 줌
@Measurement(iterations = 5, time = 1)
@Fork(1)
public class DeadCodeEliminationBenchmark {

    /** 매 호출 입력. JIT 가 상수 폴딩으로 결과를 미리 계산해 버리지 않도록 setUp 에서 주입한다. */
    private int n;

    /**
     * 노트 §2 해법 재현 — volatile 인스턴스 변수.
     * JIT 입장에서 "이 값은 언제든 다른 스레드가 읽을 수 있다"가 되어, 쓰기를 제거하지 못한다.
     * 따라서 그 값을 만드는 fib 계산도 살아남는다.
     */
    private volatile long sink;

    @Setup
    public void setUp() {
        // 상수를 직접 쓰면 컴파일러가 결과를 컴파일 타임에 접어버릴 수 있어,
        // 필드로 주입해 런타임 입력으로 만든다. JDK 의 -Dn 같은 외부 주입과 같은 취지.
        // 루프 횟수를 충분히 키워야 dead code 제거 시 사라지는 비용이 노이즈 위로 드러난다.
        n = 10_000;
    }

    /**
     * naive — 노트 §2 첫 코드(doTest)의 함정 재현.
     * 결과 r 을 지역 변수에 담고 메서드를 끝낸다. 아무도 r 을 읽지 않으므로
     * JIT 는 computeWork(n) 을 인라인해 펼친 뒤 "결과가 안 쓰인다"를 확인하고
     * 계산 전체를 제거(dead code elimination)해도 된다고 판단할 수 있다.
     * → 측정값이 실제 계산 비용이 아니라 0(빈 루프)에 가까워진다.
     */
    @Benchmark
    public void deadCodeNaive() {
        long r = computeWork(n);
        // r 을 어디서도 읽지 않음 → dead code. 노트가 경고하는 바로 그 형태.
    }

    /**
     * Blackhole 로 소비 — JMH 가 권장하는 표준 소비 방식.
     * bh.consume(r) 가 "이 값은 외부로 나간다"를 보장해 JIT 가 계산을 제거하지 못한다.
     */
    @Benchmark
    public void consumedByBlackhole(Blackhole bh) {
        long r = computeWork(n);
        bh.consume(r);
    }

    /**
     * volatile 필드로 소비 — 노트 §2 가 제시한 해법.
     * volatile 쓰기는 제거 불가 → computeWork 계산이 살아남는다. Blackhole 과 같은 효과를
     * 프레임워크 없이 손으로 구현한 형태다.
     */
    @Benchmark
    public void consumedByVolatile() {
        sink = computeWork(n);
    }

    /**
     * 단순 산술 루프 — 일부러 비용이 드는 계산. 노트 §2 의 fibImpl1 처럼 *인라인 가능한* 형태라,
     * JIT 가 호출부에 펼친 뒤 결과 사용 여부를 보고 제거를 판단할 수 있다.
     * (재귀나 @CompilerControl(DONT_INLINE) 로 인라인을 막으면, JIT 가 메서드 내부를 못 봐
     *  "부작용이 있을 수 있다"고 보수적으로 가정해 호출을 제거하지 못한다 — 그러면 dead code 제거가
     *  일어나지 않아 naive 와 소비 버전이 똑같이 나온다. 이 PoC 의 첫 시도가 정확히 그 함정이었다.)
     */
    private long computeWork(int rounds) {
        long acc = 0;
        for (int i = 0; i < rounds; i++) {
            acc += (long) (i * 31) ^ acc;
            acc = (acc << 1) | (acc >>> 63);   // 회전 — 옵티마이저가 쉽게 접지 못하게
        }
        return acc;
    }

    public static void main(String[] args) throws RunnerException {
        Options opt = new OptionsBuilder()
                .include(DeadCodeEliminationBenchmark.class.getSimpleName())
                .build();
        new Runner(opt).run();
    }
}
