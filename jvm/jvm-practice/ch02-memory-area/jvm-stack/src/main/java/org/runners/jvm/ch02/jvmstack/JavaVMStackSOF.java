package org.runners.jvm.ch02.jvmstack;

// 책 p.81 §2.4.2 가상 머신 스택 SOF (첫 번째 변형)
// VM 옵션: -Xss220k (책은 -Xss128k. JDK 21 최소 스택 208k 제약으로 220k 사용)
// 기대 결과: java.lang.StackOverflowError + 도달한 stack length 출력
public final class JavaVMStackSOF {

    private int stackLength = 1;

    public void stackLeak() {
        stackLength++;
        stackLeak();
    }

    public static void main(String[] args) throws Throwable {
        JavaVMStackSOF oom = new JavaVMStackSOF();
        try {
            oom.stackLeak();
        } catch (Throwable e) {
            System.out.println("stack length: " + oom.stackLength);
            throw e;
        }
    }
}
