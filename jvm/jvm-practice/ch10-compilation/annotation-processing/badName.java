public class badName {                 // 위반: 클래스가 소문자 시작
    static final int maxSize = 10;     // 위반: 상수가 ALL_CAPS 아님
    void DoWork() { }                  // 위반: 메서드가 대문자 시작
    void okMethod() { }                // 정상
}
