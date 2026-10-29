package com.acme.plugin;

import com.acme.api.Plugin;

// 플러그인 구현체. 톰캣의 webapps/appA 안 클래스에 해당한다.
// 별도 디렉터리(out/plugin)로 컴파일해, 각 실험에서 URLClassLoader 로 따로 로딩한다.
public class HelloPlugin implements Plugin {
    @Override
    public String run() {
        return "HelloPlugin.run() — loaded by " + getClass().getClassLoader();
    }
}
