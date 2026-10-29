package main;

public class Main {

  public static void main(String[] args) {
    a();
  }

  public static void a() {
    System.out.println("A executes");
    b();
  }

  public static void b() {
    System.out.println("B executes");
    a();
  }
}
