package biscepter

type healthcheckConf struct {
	Port int    `yaml:"port"`
	Type string `yaml:"type"`
}

type HealthcheckType int

const (
	HTTP HealthcheckType = iota
	Script
)

type Healthcheck struct {
	Port      int             // The port on which the healthcheck should be performed
	CheckType HealthcheckType // The type of healthcheck to be performed

	Script string // The script to run if the CheckType is Script
}

func (h Healthcheck) performHealthcheck() (bool, error) {
	panic("unimplemented")
}
