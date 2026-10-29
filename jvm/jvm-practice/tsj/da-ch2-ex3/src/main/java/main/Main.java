package main;

import java.util.ArrayList;

public class Main {

  public static void main(String[] args) {
    Decoder d = new Decoder();
    var list = new ArrayList<String>();
    list.add(null);  // adding null to make the code throw an exception
    var result = d.decode(list);
    System.out.println(result);
  }
}
