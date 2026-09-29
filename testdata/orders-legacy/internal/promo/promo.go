package promo

import "example.com/orderslegacy/internal/logger"

func Percent(log *logger.Logger, tier string) int {
	log.Printf("promo %s", tier)
	if tier == "gold" {
		return 10
	}
	return 0
}
