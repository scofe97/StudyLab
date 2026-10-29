package main;

public class Main {

  public static void main(String[] args) {
    FileManager fileManager = new FileManager();
    for (int i = 0; i < 10; i++) {
      fileManager.createFile(i);
    }
  }
}
