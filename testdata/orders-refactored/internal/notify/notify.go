package notify

import "example.com/ordersrefactored/internal/contracts"

type Email struct {
	Fields contracts.Fields
}

func (e *Email) Deliver(to, message string) error { return nil }
