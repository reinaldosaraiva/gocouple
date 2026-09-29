package catalog

import (
	"example.com/orderslegacy/internal/db"
	"example.com/orderslegacy/internal/logger"
)

type Catalog struct {
	store db.Store
	log   *logger.Logger
}

func New(store db.Store, log *logger.Logger) *Catalog {
	return &Catalog{store: store, log: log}
}

func (x *Catalog) Price(key string) string {
	x.log.Printf("catalog %s", key)
	v, _ := x.store.Get(key)
	return v
}
