package notify

import "example.com/orderslegacy/internal/logger"

type Notifier interface {
	Deliver(to, message string) error
}

type emailNotifier struct {
	log *logger.Logger
}

func (e *emailNotifier) Deliver(to, message string) error {
	e.log.Printf("mail %s", to)
	return nil
}

func Channels() []string { return []string{"email"} }
