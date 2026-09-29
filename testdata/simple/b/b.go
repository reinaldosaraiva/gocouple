package b

import "example.com/simple/c"

type Service struct{}

func Name() string { return c.Item{}.Label() }
