package a

import "example.com/simple/b"

type App struct{}

func (App) Run() string { return b.Name() }
