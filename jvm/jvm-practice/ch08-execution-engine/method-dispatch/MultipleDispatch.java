// 다중 디스패치 함정과 Visitor 이중 디스패치 우회
// ch03 03-03 노트(다중 디스패치와 가상 메서드 테이블) §1-2 실습.
//
// 1) Battle.attack(Character, Monster) 오버로드는 정적 다중 디스패치라,
//    선언 타입(Character/Monster)으로 컴파일 때 고정 → "기본 공격"만 불린다.
// 2) attackedBy(this) 이중 디스패치(단일 디스패치 2번)로 두 실제 타입 조합을 잡는다.
public class MultipleDispatch {

    // ── 도메인 ──────────────────────────────────────────────
    static abstract class Character {
        abstract void attack(Monster monster);
    }

    static class Warrior extends Character {
        @Override
        void attack(Monster monster) {
            // 2차 디스패치: 수신자 monster 의 실제 타입을 동적으로 푼다
            monster.attackedBy(this);
        }
    }

    static class Mage extends Character {
        @Override
        void attack(Monster monster) {
            monster.attackedBy(this);
        }
    }

    static abstract class Monster {
        abstract void attackedBy(Warrior warrior);
        abstract void attackedBy(Mage mage);
    }

    static class Slime extends Monster {
        @Override void attackedBy(Warrior warrior) { System.out.println("전사가 슬라임을 공격"); }
        @Override void attackedBy(Mage mage)       { System.out.println("마법사가 슬라임을 공격"); }
    }

    static class Dragon extends Monster {
        @Override void attackedBy(Warrior warrior) { System.out.println("전사가 드래곤을 공격"); }
        @Override void attackedBy(Mage mage)       { System.out.println("마법사가 드래곤을 공격"); }
    }

    // ── 함정: 오버로드로 두 타입 조합을 잡으려는 시도 ──────────
    static class Battle {
        void attack(Warrior warrior, Slime slime)   { System.out.println("전사가 슬라임을 공격"); }
        void attack(Warrior warrior, Dragon dragon) { System.out.println("전사가 드래곤을 공격"); }
        void attack(Mage mage, Slime slime)         { System.out.println("마법사가 슬라임을 공격"); }
        void attack(Mage mage, Dragon dragon)       { System.out.println("마법사가 드래곤을 공격"); }
        void attack(Character character, Monster monster) { System.out.println("기본 공격"); }
    }

    public static void main(String[] args) {
        // ① 오버로드 함정 — 선언 타입은 Character/Monster 라 "기본 공격"만 불린다
        Battle battle = new Battle();
        Character c = new Warrior();
        Monster m = new Dragon();
        System.out.print("오버로드: ");
        battle.attack(c, m);            // 기대 "전사가 드래곤을 공격" → 실제 "기본 공격"

        // ② 이중 디스패치 — 두 실제 타입 조합을 정확히 잡는다
        System.out.print("이중 디스패치: ");
        c.attack(m);                    // 전사가 드래곤을 공격
    }
}
