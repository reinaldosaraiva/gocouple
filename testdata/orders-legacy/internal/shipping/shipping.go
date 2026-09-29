package shipping

import "example.com/orderslegacy/internal/logger"

type Carrier struct {
	Name string
}

func (c Carrier) Quote(log *logger.Logger, grams int) int {
	log.Printf("quote %s %d", c.Name, grams)
	return grams / 10
}
