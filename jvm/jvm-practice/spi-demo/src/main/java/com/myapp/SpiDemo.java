package com.myapp;

import com.mylib.spi.PaymentProvider;
import java.io.BufferedReader;
import java.io.InputStreamReader;
import java.net.URL;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.Enumeration;
import java.util.List;
import java.util.ServiceLoader;

// 라이브러리가 구현체를 찾아 쓰는 코드.
// 핵심: 이 파일 어디에도 TossPayment / KakaoPayment 가 import 되지 않는다.
// ServiceLoader 가 META-INF/services 등록 파일을 읽어 구현체를 찾아 준다.
//
// 실행:
//   ./gradlew :spi-demo:run --args="toss 10000"     # ServiceLoader 방식
//   ./gradlew :spi-demo:run --args="kakao 5000"
//   ./gradlew :spi-demo:run --args="manual"          # getResources 로 직접 읽기(원리 학습)
public class SpiDemo {

    // ServiceLoader 가 내부에서 찾는 등록 파일 경로 규칙: "META-INF/services/" + 인터페이스 FQCN
    private static final String SERVICE_FILE =
            "META-INF/services/" + PaymentProvider.class.getName();

    public static void main(String[] args) throws Exception {
        // "manual" 모드면 ServiceLoader 없이 META-INF 를 직접 읽어 원리를 본다.
        if (args.length > 0 && args[0].equals("manual")) {
            runManual();
            return;
        }

        String type   = args.length > 0 ? args[0] : "toss";
        long   amount = args.length > 1 ? Long.parseLong(args[1]) : 10000;

        // 1) 등록된 모든 구현체를 로딩 (구현체 이름을 코드가 모름)
        ServiceLoader<PaymentProvider> providers = ServiceLoader.load(PaymentProvider.class);

        System.out.println("== 등록된 PaymentProvider 목록 (ServiceLoader) ==");
        int count = 0;
        for (PaymentProvider p : providers) {
            count++;
            System.out.println("  발견: " + p.getClass().getName());
        }
        System.out.println("  총 " + count + "개 (META-INF/services 등록 파일 기준)");
        System.out.println();

        // 2) 요청 타입(toss/kakao)을 지원하는 구현체를 골라 결제
        System.out.println("== '" + type + "' 결제 요청 ==");
        boolean handled = false;
        for (PaymentProvider p : providers) {
            if (p.supports(type)) {
                System.out.println("  " + p.pay(amount));
                handled = true;
            }
        }
        if (!handled) {
            System.out.println("  '" + type + "' 를 지원하는 제공자가 없습니다.");
        }
    }

    // ─────────────────────────────────────────────────────────────
    // ServiceLoader 가 내부에서 하는 일을 손으로 재현한다.
    // 이걸 보면 ServiceLoader 가 "META-INF 를 읽는 코드의 래퍼" 임이 드러난다.
    // ─────────────────────────────────────────────────────────────
    static void runManual() throws Exception {
        ClassLoader cl = Thread.currentThread().getContextClassLoader();
        System.out.println("== manual: getResources 로 META-INF 직접 읽기 ==");
        System.out.println("찾는 파일: " + SERVICE_FILE);
        System.out.println();

        // 1) 클래스 로더에게 등록 "파일" 자체를 요청한다. (ServiceLoader 의 핵심 한 줄)
        //    클래스패스 여러 곳에 같은 파일이 있을 수 있어 Enumeration 으로 돌려준다.
        Enumeration<URL> urls = cl.getResources(SERVICE_FILE);

        List<PaymentProvider> found = new ArrayList<>();
        while (urls.hasMoreElements()) {
            URL url = urls.nextElement();
            System.out.println("  등록 파일 위치: " + url);

            // 2) 파일을 한 줄씩 읽어 구현체 FQCN 을 얻는다. (# 주석·빈 줄 건너뜀)
            for (String line : readLines(url)) {
                String name = stripComment(line).trim();
                if (name.isEmpty()) continue;
                System.out.println("    → 등록된 구현체 이름: " + name);

                // 3) 그 이름으로 클래스를 로딩하고 인스턴스를 만든다.
                Class<?> impl = Class.forName(name, true, cl);
                PaymentProvider p = (PaymentProvider) impl.getDeclaredConstructor().newInstance();
                found.add(p);
            }
        }

        System.out.println();
        System.out.println("  직접 읽어 만든 구현체 " + found.size() + "개:");
        for (PaymentProvider p : found) {
            System.out.println("    " + p.getClass().getName() + " → " + p.pay(1000));
        }
        System.out.println();
        System.out.println("  ↑ ServiceLoader.load() 가 내부에서 정확히 이 일을 한다.");
    }

    // URL 이 가리키는 텍스트 파일을 줄 리스트로 읽는다.
    static List<String> readLines(URL url) throws Exception {
        List<String> lines = new ArrayList<>();
        try (BufferedReader r = new BufferedReader(
                new InputStreamReader(url.openStream(), StandardCharsets.UTF_8))) {
            String line;
            while ((line = r.readLine()) != null) {
                lines.add(line);
            }
        }
        return lines;
    }

    // '#' 이후를 주석으로 잘라낸다 (SPI 파일 규칙).
    static String stripComment(String line) {
        int hash = line.indexOf('#');
        return hash >= 0 ? line.substring(0, hash) : line;
    }
}
