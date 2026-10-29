// 실무 함정: 이벤트 핸들러 오버로딩이 정적 디스패치라 의도와 다르게 불린다
// ch03 03-02 노트(메서드 호출 — 해석과 정적·동적 디스패치) §실무 실습.
//
// process(Event) / process(UserCreatedEvent) 오버로드를 Event 선언 타입으로 호출하면
// 컴파일러가 선언 타입 Event 만 보고 process(Event) 를 골라 박는다(정적 디스패치).
// 회피: 이벤트 객체에 행위를 위임하면 오버라이딩(동적 디스패치)이라 정상 작동한다.
public class EventDispatch {

    interface Event {
        void process();                 // 회피책: 다형 위임
    }

    static class UserCreatedEvent implements Event {
        @Override public void process() { System.out.println("사용자 생성 처리"); }
    }

    static class OrderCreatedEvent implements Event {
        @Override public void process() { System.out.println("주문 생성 처리"); }
    }

    // ── 함정: 타입별 오버로드 ───────────────────────────────
    static class Dispatcher {
        void handle(Event event) {
            process(event);             // 선언 타입 Event → 항상 process(Event)
        }
        void process(UserCreatedEvent event) { System.out.println("사용자 생성 처리"); }
        void process(OrderCreatedEvent event) { System.out.println("주문 생성 처리"); }
        void process(Event event) { System.out.println("기본 처리"); }
    }

    public static void main(String[] args) {
        Event event = new UserCreatedEvent();

        // ① 오버로드 함정 — 실제 타입은 UserCreatedEvent 인데 "기본 처리"가 불린다
        Dispatcher dispatcher = new Dispatcher();
        System.out.print("오버로드 핸들러: ");
        dispatcher.handle(event);       // 기대 "사용자 생성 처리" → 실제 "기본 처리"

        // ② 다형 위임 — 동적 디스패치라 실제 타입 버전이 불린다
        System.out.print("다형 위임: ");
        event.process();                // 사용자 생성 처리
    }
}
