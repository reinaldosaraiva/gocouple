package payment

import "example.com/ordersrefactored/internal/contracts"

type StripeGateway struct {
	Fields contracts.Fields
}

func (g *StripeGateway) Charge(cents int) error { return nil }
