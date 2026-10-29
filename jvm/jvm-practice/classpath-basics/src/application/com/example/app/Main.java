package com.example.app;

import com.example.math.Calculator;

import java.net.URL;

public final class Main {
    private Main() {
    }

    public static void main(String[] args) {
        System.out.println("20 + 22 = " + Calculator.add(
                20,
                22));
        System.out.println("Main.class       <- " + loadedFrom(Main.class));
        System.out.println("Calculator.class <- " + Calculator.loadedFrom());
    }

    private static URL loadedFrom(Class<?> type) {
        return type
                .getProtectionDomain()
                .getCodeSource()
                .getLocation();
    }
}
