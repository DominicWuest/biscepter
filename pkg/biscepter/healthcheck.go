package biscepter

import (
	"fmt"
	"net/http"
)

type healthcheckConf struct {
	Port int    `yaml:"port"`
	Type string `yaml:"type"`
}

type HealthcheckType int

const (
	// Healthcheck consists of a single http GET request. Healthcheck metadata holds the path to which the request is sent
	HttpGet200 HealthcheckType = iota
	Script
)

type Healthcheck struct {
	Port      int             // The port on which the healthcheck should be performed
	CheckType HealthcheckType // The type of healthcheck to be performed

	// TODO: Find better name
	Metadata string // Additional metadata for a given check type. Functionality depends on check type
}

func (h Healthcheck) performHealthcheck(portsMapping map[int]int) (bool, error) {
	switch h.CheckType {
	case HttpGet200:
		res, err := http.Get(fmt.Sprintf("http://localhost:%d%s", portsMapping[h.Port], h.Metadata))
		if err != nil {
			return false, err
		}
		return res.StatusCode == 200, nil
	default:
		panic("unimplemented")
	}
}
