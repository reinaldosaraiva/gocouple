package contracts

import (
	"time"

	"example.com/orderslegacy/internal/customer"
)

type CustomerLookup interface {
	Find(id string) customer.Customer
}

type Clock interface {
	Now() time.Time
}
