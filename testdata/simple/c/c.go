package c

type Store interface {
	Get(key string) string
}

type Item struct{}

func (Item) Label() string { return "item" }

type Number interface {
	~int | ~float64
}

func Sum[T Number](xs ...T) T {
	var total T
	for _, x := range xs {
		total += x
	}
	return total
}
