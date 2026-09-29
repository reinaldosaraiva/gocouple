package main

import (
	"os"

	"example.com/ordersrefactored/internal/catalog"
	"example.com/ordersrefactored/internal/logger"
	"example.com/ordersrefactored/internal/notify"
	"example.com/ordersrefactored/internal/order"
	"example.com/ordersrefactored/internal/payment"
)

func main() {
	svc := &order.Service{
		Catalog: &catalog.Static{Prices: map[string]int{"sku-1": 100}},
		Gateway: &payment.StripeGateway{},
		Notify:  &notify.Email{},
		Log:     logger.New(),
	}
	if err := svc.Place("ana", "sku-1"); err != nil {
		os.Exit(1)
	}
}
