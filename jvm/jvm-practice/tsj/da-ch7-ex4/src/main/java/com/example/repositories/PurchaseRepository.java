package com.example.repositories;

import com.example.entities.Product;
import com.example.entities.Purchase;
import org.springframework.stereotype.Repository;

import javax.persistence.EntityManager;
import javax.persistence.TypedQuery;
import javax.persistence.criteria.CriteriaBuilder;
import javax.persistence.criteria.CriteriaQuery;
import javax.persistence.criteria.Predicate;
import javax.persistence.criteria.Root;
import java.util.List;

@Repository
public class PurchaseRepository {

  private final EntityManager entityManager;

  public PurchaseRepository(EntityManager entityManager) {
    this.entityManager = entityManager;
  }

  public List<Purchase> findAll() {
    CriteriaBuilder cb = entityManager.getCriteriaBuilder();
    CriteriaQuery<Purchase> cq = cb.createQuery(Purchase.class);

    cq.from(Purchase.class);

    TypedQuery<Purchase> query = entityManager.createQuery(cq);
    return query.getResultList();
  }
}