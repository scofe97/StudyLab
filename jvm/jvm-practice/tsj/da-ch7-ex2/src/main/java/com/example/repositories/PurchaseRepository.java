package com.example.repositories;

import com.example.model.Purchase;
import com.example.repositories.mappers.PurchaseRowMapper;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.stereotype.Repository;

import java.util.HashSet;
import java.util.List;
import java.util.Set;

@Repository
public class PurchaseRepository {

  private final JdbcTemplate jdbcTemplate;

  public PurchaseRepository(JdbcTemplate jdbcTemplate) {
    this.jdbcTemplate = jdbcTemplate;
  }

  public List<Purchase> findAll() {
    String sql = "SELECT * FROM purchase";
    return jdbcTemplate.query(sql, new PurchaseRowMapper());
  }

  public Set<String> findPurchasedProductNames() {
    String sql = "SELECT DISTINCT p.name FROM purchase pu JOIN product p ON pu.product = p.id";
    return new HashSet<>(jdbcTemplate.queryForList(sql, String.class));
  }
}
