package order

import (
	"example.com/orderslegacy/internal/audit"
	"example.com/orderslegacy/internal/catalog"
	"example.com/orderslegacy/internal/customer"
	"example.com/orderslegacy/internal/db"
	"example.com/orderslegacy/internal/logger"
	"example.com/orderslegacy/internal/notify"
	"example.com/orderslegacy/internal/payment"
	"example.com/orderslegacy/internal/promo"
	"example.com/orderslegacy/internal/shipping"
	"example.com/orderslegacy/internal/stock"
)

type Service struct {
	catalog *catalog.Catalog
	stock   *stock.Stock
	audit   *audit.Audit
	gateway *payment.StripeGateway
	carrier shipping.Carrier
	log     *logger.Logger
}

func New() *Service {
	log := logger.New("orders")
	store := db.New(log)
	return &Service{
		catalog: catalog.New(store, log),
		stock:   stock.New(store, log),
		audit:   audit.New(store, log),
		gateway: &payment.StripeGateway{Log: log},
		carrier: shipping.Carrier{Name: "post"},
		log:     log,
	}
}

func (s *Service) Place(c customer.Customer, sku string, grams int) error {
	price := len(s.catalog.Price(sku)) + s.carrier.Quote(s.log, grams)
	price -= promo.Percent(s.log, c.Tier)
	_ = s.stock.Level(sku)
	if err := s.gateway.Charge(price); err != nil {
		return err
	}
	s.audit.Record(sku)
	s.log.Printf("channels %v", notify.Channels())
	return nil
}
