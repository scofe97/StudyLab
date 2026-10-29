// 실습 ① 3계층 로더 확인 — getClassLoader() 로 어느 로더가 로딩했나 본다.
//   부트스트랩(java.*)은 null, 내 클래스는 AppClassLoader.
//
// 실행: javac LoaderInspect.java && java LoaderInspect
//   기대:
//     String           → null          (부트스트랩 — 자바 객체 아님)
//     LoaderInspect    → AppClassLoader (애플리케이션)
//     로더 부모 체인    → App → Platform → null(부트스트랩)
public class LoaderInspect {
    public static void main(String[] args) {
        // 핵심 클래스 — 부트스트랩이 로딩 → null
        System.out.println("String 로더      = " + String.class.getClassLoader());
        System.out.println("ArrayList 로더   = " + java.util.ArrayList.class.getClassLoader());

        // 내가 짠 클래스 — 애플리케이션 로더
        System.out.println("내 클래스 로더    = " + LoaderInspect.class.getClassLoader());

        // 로더 부모 체인 — 위로 올라가며 출력 (위임 방향)
        System.out.println("\n-- 로더 부모 체인 (자식 → 부모) --");
        ClassLoader cl = LoaderInspect.class.getClassLoader();
        while (cl != null) {
            System.out.println("  " + cl);
            cl = cl.getParent();
        }
        System.out.println("  null  ← 부트스트랩 (C++ 구현이라 자바 객체 없음)");
    }
}
