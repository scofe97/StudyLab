package com.example.math;

import java.net.URL;

public final class Calculator {
    private Calculator() {
    }

    public static int add(
            int left,
            int right) {
        return left + right;
    }

    public static URL loadedFrom() {
        return Calculator.class
                .getProtectionDomain()
                .getCodeSource()
                .getLocation();
    }
}
