public class EscapeDemo {
    static class Point { int x, y; Point(int x, int y){ this.x=x; this.y=y; } }

    // 비탈출 객체를 1억 번 생성 — 탈출분석 ON이면 스칼라 치환으로 객체가 안 만들어짐
    static long sumDistances() {
        long sum = 0;
        for (int i = 0; i < 100_000_000; i++) {
            Point p = new Point(i, i + 1);   // p 는 메서드 밖으로 안 샘 (비탈출)
            sum += (long) p.x * p.x + (long) p.y * p.y;  // 안에서만 읽음
        }
        return sum;
    }

    public static void main(String[] args) {
        long start = System.nanoTime();
        long result = sumDistances();
        long ms = (System.nanoTime() - start) / 1_000_000;
        System.out.println("result=" + result + " (" + ms + " ms)");
    }
}
