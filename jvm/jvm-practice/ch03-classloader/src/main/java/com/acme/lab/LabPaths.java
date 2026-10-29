package com.acme.lab;

import java.io.File;
import java.net.MalformedURLException;
import java.net.URL;

// 실험들이 로딩할 "별도 컴파일 산출물" 디렉터리의 베이스 경로를 한 곳에서 결정한다.
//
// 왜 필요한가: HelloPlugin·Greeter(v1/v2)·provider 는 앱 클래스패스에 두면 안 되고
// (두면 격리 실험이 깨진다) URLClassLoader 로 *별도 디렉터리에서* 로딩해야 한다.
// 그 디렉터리 위치가 실행 방식마다 다르다.
//   - scripts/run.sh: 작업 디렉터리에서 out/ 아래 (기본값)
//   - Gradle/IntelliJ: -Dlab.out=<build 경로> 로 주입
//
// 시스템 프로퍼티 lab.out 이 있으면 그걸, 없으면 "out" 을 쓴다.
public final class LabPaths {
    private LabPaths() {}

    private static final String BASE = System.getProperty("lab.out", "out");

    /** 베이스 아래 하위 디렉터리를 URLClassLoader 용 URL 로 돌려준다. (예: dir("v1")) */
    public static URL dir(String name) {
        try {
            return new File(BASE, name).toURI().toURL();
        } catch (MalformedURLException e) {
            throw new IllegalStateException("lab 경로 변환 실패: " + BASE + "/" + name, e);
        }
    }
}
