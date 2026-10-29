package com.example.repositories;

import com.example.model.Product;
import com.example.model.Purchase;
import com.example.repositories.mappers.PurchaseRowMapper;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.stereotype.Repository;

import javax.sql.DataSource;
import java.sql.Connection;
import java.sql.DriverManager;
import java.sql.PreparedStatement;
import java.sql.ResultSet;
import java.sql.SQLException;
import java.util.ArrayList;
import java.util.List;

@Repository
public class PurchaseRepository {

  private final DataSource dataSource;

  public PurchaseRepository(DataSource dataSource) {
    this.dataSource = dataSource;
  }

  public List<Purchase> findAll() throws SQLException {
    List<Purchase> list = new ArrayList<>();
    String sql = "SELECT * FROM purchase";

    try (Connection con = dataSource.getConnection();
         PreparedStatement statement = con.prepareStatement(sql)) {

      ResultSet result = statement.executeQuery();

      while (result.next()) {
        Purchase p = new Purchase();
        p.setId(result.getInt("id"));
        p.setPrice(result.getBigDecimal("price"));
        p.setProduct(result.getInt("product"));
        list.add(p);
      }
    }
    return list;
  }
}
