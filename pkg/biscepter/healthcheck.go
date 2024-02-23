package biscepter

import (
	"fmt"
	"net/http"
	"time"
)

type healthcheckYaml struct {
	Port int    `yaml:"port"`
	Type string `yaml:"type"`

	Metadata string `yaml:"metadata"`

	Retries int `yaml:"retries" default:"10"`

	Backoff          time.Duration `yaml:"backoff" default:"1000"`
	BackoffIncrement time.Duration `yaml:"backoffIncrement" default:"100"`
	MaxBackoff       time.Duration `yaml:"maxBackoff" default:"2000"`
}

// HealthcheckConfig provides configurations for healthchecks being performed, such as the amount of retries or backoff duration
type HealthcheckConfig struct {
	Retries int // How many times this healthcheck should be retried until it is considered to have failed

	Backoff time.Duration // How long to wait between each healthcheck retry

	BackoffIncrement time.Duration // By how much to increment the backoff on each failed attempt
	MaxBackoff       time.Duration // The maximum duration the backoff may reach after incrementing. When the backoff has reached this value, it won't increase any further
}

// TODO: Docs
type HealthcheckType int

const (
	// Healthcheck consists of a single http GET request. Healthcheck metadata holds the path to which the request is sent
	HttpGet200 HealthcheckType = iota
	Script
)

// TODO: Docs
type Healthcheck struct {
	Port      int             // The port on which the healthcheck should be performed
	CheckType HealthcheckType // The type of healthcheck to be performed

	// TODO: Find better name
	Metadata string            // Additional metadata for a given check type. Functionality depends on check type
	Config   HealthcheckConfig // The config for this healthcheck
}

// performHealthcheck performs the given healthcheck of the passed port mappings.
// If the healthcheck is unsuccessful, the returned boolean is false and the error may not be nil.
// If the returned boolean is true, the returned error is nil
func (h Healthcheck) performHealthcheck(portsMapping map[int]int) (bool, error) {
	var lastSuccess bool
	var lastError error

	backoffDuration := h.Config.Backoff
	for i := 0; i < h.Config.Retries; i++ {
		lastSuccess, lastError = h.performSingleHealthcheck(portsMapping)

		// Manage backoff
		if (i != h.Config.Retries-1) && !lastSuccess {
			time.Sleep(backoffDuration)
			backoffDuration += h.Config.BackoffIncrement
			if backoffDuration > h.Config.MaxBackoff {
				backoffDuration = h.Config.MaxBackoff
			}
		}
	}

	return lastSuccess, lastError
}

// performHealthcheck performs a single try of the given healthcheck of the passed port mappings.
// If the healthcheck is unsuccessful, the returned boolean is false and the error may not be nil.
// If the returned boolean is true, the returned error is nil
func (h Healthcheck) performSingleHealthcheck(portsMapping map[int]int) (bool, error) {
	switch h.CheckType {
	case HttpGet200:
		res, err := http.Get(fmt.Sprintf("http://localhost:%d%s", portsMapping[h.Port], h.Metadata))
		if err != nil {
			return false, err
		}
		return res.StatusCode == 200, nil
	// TODO: Implement more healthchecks
	default:
		panic("unimplemented")
	}
}
