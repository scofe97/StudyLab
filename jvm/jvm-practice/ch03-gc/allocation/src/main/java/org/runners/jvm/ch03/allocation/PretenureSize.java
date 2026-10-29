package org.runners.jvm.ch03.allocation;

public final class PretenureSize {

    private static final int _1MB = 1024 * 1024;

    public static void main(String[] args) {
        byte[] big = new byte[4 * _1MB];

        System.out.println("PretenureSize done. GC 로그는 build/pretenure-gc.log 에서 확인");
        System.out.println("alive: " + big.length + " bytes");
    }
}
