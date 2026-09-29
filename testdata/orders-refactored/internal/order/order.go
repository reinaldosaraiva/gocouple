package order

import "example.com/ordersrefactored/internal/contracts"

type Service struct {
	Catalog contracts.Catalog
	Gateway contracts.Gateway
	Notify  contracts.Notifier
	Log     contracts.Logger
}

func (s *Service) Place(customer, sku string) error {
	price := s.Catalog.Price(sku)
	if err := s.Gateway.Charge(price); err != nil {
		return err
	}
	s.Log.Printf("placed %s for %s", sku, customer)
	return s.Notify.Deliver(customer, "order placed")
}
