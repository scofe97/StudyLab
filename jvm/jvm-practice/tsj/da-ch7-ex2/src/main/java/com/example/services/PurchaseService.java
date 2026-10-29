package com.example.services;

import com.example.repositories.PurchaseRepository;
import org.springframework.stereotype.Service;

import java.util.Set;

@Service
public class PurchaseService {

  private final PurchaseRepository purchaseRepository;

  public PurchaseService(PurchaseRepository purchaseRepository) {
    this.purchaseRepository = purchaseRepository;
  }

  public Set<String> getProductNamesForPurchases() {
    return purchaseRepository.findPurchasedProductNames();
  }
}
