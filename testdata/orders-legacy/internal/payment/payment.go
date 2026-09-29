package payment

import "example.com/orderslegacy/internal/logger"

type Gateway interface {
	Charge(cents int) error
}

type StripeGateway struct {
	Log *logger.Logger
}

func (g *StripeGateway) Charge(cents int) error {
	g.Log.Printf("charge %d", cents)
	return nil
}
