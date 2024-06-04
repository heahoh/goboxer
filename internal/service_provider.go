package internal

type ServiceProvider interface {
	GetServicesCnt() int
	GetServicesList() map[string]*ServiceConfig
}

type StubServiceProvider struct {
	Booted          bool
	Cnt             int
	ServicesConfigs map[string]*ServiceConfig
}

func (p *StubServiceProvider) GetServicesCnt() int {
	if !p.Booted {
		p.Boot()
	}
	return p.Cnt
}

func (p *StubServiceProvider) GetServicesList() map[string]*ServiceConfig {
	if !p.Booted {
		p.Boot()
	}
	return p.ServicesConfigs
}

func (p *StubServiceProvider) Boot() {
	p.Cnt = 1
	p.ServicesConfigs = make(map[string]*ServiceConfig, p.Cnt)
	p.ServicesConfigs["Any"] = &ServiceConfig{
		Dsn:    "root:2eXFSPmS@tcp(127.0.0.1:3317)/shopping-delivery",
		Driver: "mysql",
	}
	p.ServicesConfigs["Other"] = &ServiceConfig{
		Dsn:    "root:2eXFSPmS@tcp(127.0.0.1:3317)/shopping-delivery",
		Driver: "mysql",
	}
	p.Booted = true
}
