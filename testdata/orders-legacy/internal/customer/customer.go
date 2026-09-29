package customer

import (
	"example.com/orderslegacy/internal/logger"
	"example.com/orderslegacy/internal/promo"
)

type Customer struct {
	ID   string
	Tier string
}

func (c Customer) Discount(log *logger.Logger) int {
	return promo.Percent(log, c.Tier)
}
