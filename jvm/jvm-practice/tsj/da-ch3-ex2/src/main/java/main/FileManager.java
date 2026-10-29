package main;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Paths;

public class FileManager {

  public boolean createFile(int i) {
    try {
      Files.createFile(Paths.get("File " + i));
      return true;
    } catch (IOException e) {
      e.printStackTrace();
    }
    return false;
  }
}
