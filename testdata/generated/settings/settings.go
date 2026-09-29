package settings

type Settings struct{ Addr string }

func Load() Settings { return Settings{Addr: ":8080"} }
