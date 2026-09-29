package stock

import (
	"example.com/orderslegacy/internal/db"
	"example.com/orderslegacy/internal/logger"
)

type Stock struct {
	store db.Store
	log   *logger.Logger
}

func New(store db.Store, log *logger.Logger) *Stock {
	return &Stock{store: store, log: log}
}

func (x *Stock) Level(key string) string {
	x.log.Printf("stock %s", key)
	v, _ := x.store.Get(key)
	return v
}
