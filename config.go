package main

type Config struct {
	scanner bool
	pool    bool
	pay     bool
    web     bool
}

func NewConfig(scanner, pool, pay, web, all bool) *Config {
	return &Config{scanner || all,
                   pool || all,
                   pay || all,
                   web || all,
               }
}

func (self *Config) OnlyScanner() bool {
	return self.scanner && !self.pool && !self.pay
}
