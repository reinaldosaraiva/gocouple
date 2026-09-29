package audit

import (
	"example.com/orderslegacy/internal/db"
	"example.com/orderslegacy/internal/logger"
)

type Audit struct {
	store db.Store
	log   *logger.Logger
}

func New(store db.Store, log *logger.Logger) *Audit {
	return &Audit{store: store, log: log}
}

func (x *Audit) Record(key string) string {
	x.log.Printf("audit %s", key)
	v, _ := x.store.Get(key)
	return v
}
