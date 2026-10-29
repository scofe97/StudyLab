public class Demo {
    public static void main(String[] args) {
        User u = new User();
        u.name = "보현";
        u.age = 30;
        // UserGetters 는 우리가 안 썼다 — 처리기가 컴파일 때 생성한 클래스
        System.out.println(UserGetters.getName(u));
        System.out.println(UserGetters.getAge(u));
    }
}
