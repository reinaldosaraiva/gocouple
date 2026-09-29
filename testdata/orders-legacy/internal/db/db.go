package db

import "example.com/orderslegacy/internal/logger"

type Store interface {
	Put(key, value string)
	Get(key string) (string, bool)
}

type memory struct {
	log  *logger.Logger
	data map[string]string
}

func New(log *logger.Logger) Store {
	return &memory{log: log, data: map[string]string{}}
}

func (m *memory) Put(key, value string) {
	m.log.Printf("put %s", key)
	m.data[key] = value
}

func (m *memory) Get(key string) (string, bool) {
	v, ok := m.data[key]
	return v, ok
}
