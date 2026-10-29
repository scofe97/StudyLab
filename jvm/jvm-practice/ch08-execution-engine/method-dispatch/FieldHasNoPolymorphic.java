// 실습 ④ 메서드는 오버라이딩(동적), 필드는 가려짐(정적) — 비대칭을 한 번에 본다.
//
// 실행:
//   javac FieldHasNoPolymorphic.java
//   java FieldHasNoPolymorphic
//     → I am Son, I have $0       (메서드는 동적: Father 생성자에서 불러도 Son 버전)
//       I am Son, I have $4       (Son 생성자에서 다시, 이번엔 money=4 초기화됨)
//       This guy has $2           (필드는 정적: guy 의 선언 타입 Father 의 money)
//
//   javap -c FieldHasNoPolymorphic
//     - showMeMoney 호출은 invokevirtual (동적 디스패치)
//     - guy.money 접근은 getfield ... Father.money:I  (정적 타입 Father 로 박힘!)
//
//   포인트:
//   1) 메서드: Father 생성자의 showMeMoney() 가 실제 타입 Son 의 것을 부른다(동적).
//      첫 호출 때는 Son 의 money 가 아직 초기화 전이라 0 이 보인다(초기화 순서 함정).
//   2) 필드: guy.money 는 guy 의 *선언 타입* Father 를 따라 Father.money(=2) 가 보인다.
//      같은 객체인데 변수 선언 타입에 따라 다른 필드가 보이는 것이 "필드 가려짐(hiding)".
//   → 메서드는 실제 타입, 필드는 선언(정적) 타입. 이 비대칭이 핵심.
public class FieldHasNoPolymorphic {

    static class Father {
        public int money = 1;
        public Father() {
            money = 2;
            showMeMoney();        // 생성자에서 오버라이드된 메서드 호출
        }
        public void showMeMoney() {
            System.out.println("I am Father, I have $" + money);
        }
    }

    static class Son extends Father {
        public int money = 3;
        public Son() {
            money = 4;
            showMeMoney();
        }
        @Override
        public void showMeMoney() {
            System.out.println("I am Son, I have $" + money);
        }
    }

    public static void main(String[] args) {
        Father guy = new Son();
        System.out.println("This guy has $" + guy.money);   // 어느 money 가 보일까?
    }
}
