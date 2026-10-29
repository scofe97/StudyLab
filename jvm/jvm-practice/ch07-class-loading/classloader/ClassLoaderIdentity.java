// 실습 ② 클래스 동일성 — 같은 .class 라도 다른 로더로 로딩하면 instanceof false.
//   노트 §1 ClassLoaderTest 를 단순화. 사용자 정의 로더가 이 클래스 자신을
//   다시 로딩하면, 같은 바이트인데도 JVM 에겐 별개의 클래스다.
//
// 실행: javac ClassLoaderIdentity.java && java ClassLoaderIdentity
//   기대:
//     obj.getClass()                      → class ClassLoaderIdentity (이름은 같음)
//     obj instanceof ClassLoaderIdentity  → false   (로더가 달라 별개)
import java.io.InputStream;
import java.io.IOException;

public class ClassLoaderIdentity {
    public static void main(String[] args) throws Exception {
        // 사용자 정의 로더 — 클래스 이름의 .class 를 직접 읽어 defineClass
        ClassLoader myLoader = new ClassLoader() {
            @Override
            public Class<?> loadClass(String name) throws ClassNotFoundException {
                try {
                    String fileName = name.substring(name.lastIndexOf(".") + 1) + ".class";
                    InputStream is = getClass().getResourceAsStream(fileName);
                    if (is == null) {
                        return super.loadClass(name);   // 못 찾으면 기본 위임
                    }
                    byte[] b = is.readAllBytes();
                    return defineClass(name, b, 0, b.length);  // 직접 클래스 정의
                } catch (IOException e) {
                    throw new ClassNotFoundException(name);
                }
            }
        };

        // 사용자 로더로 이 클래스 자신을 다시 로딩
        Object obj = myLoader.loadClass("ClassLoaderIdentity").getDeclaredConstructor().newInstance();

        System.out.println("obj.getClass()                     = " + obj.getClass());
        // instanceof 비교 — 같은 .class 인데?
        System.out.println("obj instanceof ClassLoaderIdentity = " + (obj instanceof ClassLoaderIdentity));
        System.out.println("\n같은 바이트인데 로더가 달라 JVM 에겐 별개 클래스 → instanceof false");
    }
}
