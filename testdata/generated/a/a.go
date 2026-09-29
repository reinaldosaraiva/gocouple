package a

import (
	"example.com/generated/gen"
	"example.com/generated/mixed"
	"example.com/generated/settings"
)

func Use() (gen.Reply, mixed.Handler, settings.Settings) {
	return gen.Reply{}, mixed.Handler{}, settings.Load()
}
