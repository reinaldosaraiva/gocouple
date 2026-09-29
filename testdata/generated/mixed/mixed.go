package mixed

type Handler struct{ Name string }

func (Handler) Serve() string { return "ok" }
