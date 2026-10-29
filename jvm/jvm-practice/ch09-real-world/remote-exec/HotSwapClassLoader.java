// 치환된 byte[] 를 적재하는 로더. 핵심은 protected 인 defineClass 를 외부에서 쓸 수 있게 연 것.
// 매 실행 요청마다 이 로더의 "새 인스턴스" 를 만들어 적재한다 — "동일성 = 이름 + 로더" 라,
// 로더가 새것이면 같은 이름의 (수정된) 클래스도 다른 클래스로 다시 로딩된다(핫스왑).
public class HotSwapClassLoader extends ClassLoader {

    public HotSwapClassLoader() {
        // 부모를 이 로더를 로딩한 로더로 지정 → HackSystem 등은 부모 위임으로 정상 해석됨
        super(HotSwapClassLoader.class.getClassLoader());
    }

    // protected defineClass 노출 — 외부(JavaClassExecuter)가 byte[] 를 직접 적재할 수 있게
    public Class<?> loadByte(byte[] classByte) {
        return defineClass(null, classByte, 0, classByte.length);
    }
}
