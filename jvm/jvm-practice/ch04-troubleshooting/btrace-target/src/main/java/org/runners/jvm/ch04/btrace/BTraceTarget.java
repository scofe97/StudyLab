package org.runners.jvm.ch04.btrace;

import java.util.Random;

// 책 p.234 §4.3.3 — VisualVM BTrace 플러그인으로 동적 추적할 대상 프로그램
// 관찰 방법:
//   1) 이 프로그램을 run 으로 띄운다(add 를 1초마다 반복 호출).
//   2) VisualVM 에서 이 프로세스에 attach → BTrace 플러그인.
//   3) add(int,int) 진입을 @OnMethod 로 가로채는 스크립트를 주입하면,
//      재시작 없이 실행 중 인자/반환값이 콘솔로 찍힌다.
// ⚠️ 무한 루프이므로 관찰 후 수동 종료한다.
public final class BTraceTarget {

    // BTrace 스크립트가 @OnMethod(method = "add") 로 가로챌 메서드
    public int add(int a, int b) {
        return a + b;
    }

    public static void main(String[] args) throws InterruptedException {
        BTraceTarget target = new BTraceTarget();
        Random random = new Random();
        while (true) {
            // 1초 간격으로 호출 — BTrace 콘솔에 추적 로그가 천천히 쌓이도록
            int result = target.add(random.nextInt(1000), random.nextInt(1000));
            System.out.println("add result = " + result);
            Thread.sleep(1000);
        }
    }
}
