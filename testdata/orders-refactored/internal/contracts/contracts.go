package contracts

type Fields map[string]string

type Logger interface {
	Printf(format string, args ...any)
	With(fields Fields) Logger
}

type Gateway interface {
	Charge(cents int) error
}

type Notifier interface {
	Deliver(to, message string) error
}

type Catalog interface {
	Price(sku string) int
}
