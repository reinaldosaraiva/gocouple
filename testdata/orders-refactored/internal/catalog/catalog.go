package catalog

import "example.com/ordersrefactored/internal/contracts"

type Static struct {
	Prices map[string]int
	Fields contracts.Fields
}

func (s *Static) Price(sku string) int { return s.Prices[sku] }
