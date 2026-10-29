package org.runners.jvm.ch01;

public final class JavaTechSystemDemo {

    public static void main(String[] args) {
        System.out.println("java.version       = " + System.getProperty("java.version"));
        System.out.println("java.vendor        = " + System.getProperty("java.vendor"));
        System.out.println("java.vm.name       = " + System.getProperty("java.vm.name"));
        System.out.println("java.vm.version    = " + System.getProperty("java.vm.version"));
        System.out.println("java.home          = " + System.getProperty("java.home"));
    }
}
