package biscepter

import (
	"fmt"
	"net/http"
	"os/exec"
	"time"

	"github.com/sirupsen/logrus"
)

type healthcheckYaml struct {
	Port int    `yaml:"port"`
	Type string `yaml:"type"`

	Data string `yaml:"data"`

	Retries int `yaml:"retries" default:"25"`

	Backoff          time.Duration `yaml:"backoff" default:"1000"`
	BackoffIncrement time.Duration `yaml:"backoffIncrement" default:"250"`
	MaxBackoff       time.Duration `yaml:"maxBackoff" default:"3000"`
}

// HealthcheckConfig provides configurations for healthchecks being performed, such as the amount of retries or backoff duration
type HealthcheckConfig struct {
	Retries int // How many times this healthcheck should be retried until it is considered to have failed

	Backoff time.Duration // How long to wait between each healthcheck retry

	BackoffIncrement time.Duration // By how much to increment the backoff on each failed attempt
	MaxBackoff       time.Duration // The maximum duration the backoff may reach after incrementing. When the backoff has reached this value, it won't increase any further
}

// HealthcheckType specifies the type of healthcheck to be performed
type HealthcheckType int

const (
	// Healthcheck consists of a single http GET request. Healthcheck data holds the path to which the request is sent
	HttpGet200 HealthcheckType = iota
	// Healthcheck consists of a custom script ran in bash. Healthcheck data holds the actual script.
	// The environment variable `$PORT<XXXX>` can be used within the script to get the port to which `<XXXX>` was mapped to on the host (e.g. `$PORT443`)
	Script
)

// The Healthcheck struct represents a healthcheck performed by the replica on running systems before sending them out to be tested
type Healthcheck struct {
	Port      int             // The port on which the healthcheck should be performed
	CheckType HealthcheckType // The type of healthcheck to be performed

	Data   string            // Additional data for a given check type. Functionality depends on check type
	Config HealthcheckConfig // The config for this healthcheck
}

// performHealthcheck performs the given healthcheck of the passed port mappings.
// If the healthcheck is unsuccessful, the returned boolean is false and the error may not be nil.
// If the returned boolean is true, the returned error is nil
func (h Healthcheck) performHealthcheck(portsMapping map[int]int, log *logrus.Entry) (bool, error) {
	var lastSuccess bool
	var lastError error

	backoffDuration := h.Config.Backoff
	for i := 0; i < h.Config.Retries; i++ {
		lastSuccess, lastError = h.performSingleHealthcheck(portsMapping)

		// Manage backoff
		if (i != h.Config.Retries-1) && !lastSuccess {
			log.Tracef("Healthcheck %d/%d failed. Error: %v. Waiting for %s", i+1, h.Config.Retries, lastError, backoffDuration.String())
			time.Sleep(backoffDuration)
			backoffDuration += h.Config.BackoffIncrement
			if backoffDuration > h.Config.MaxBackoff {
				backoffDuration = h.Config.MaxBackoff
			}
		} else {
			// Healthcheck successful
			log.Debugf("Healthcheck successful after %d tries", i+1)
			break
		}
	}

	if !lastSuccess {
		log.Warnf("Healthcheck %d/%d of type %d failed on port %d which was mapped to %d.", h.Config.Retries, h.Config.Retries, h.CheckType, h.Port, portsMapping[h.Port])
	}

	return lastSuccess, lastError
}

// performHealthcheck performs a single try of the given healthcheck of the passed port mappings.
// If the healthcheck is unsuccessful, the returned boolean is false and the error may not be nil.
// If the returned boolean is true, the returned error is nil
func (h Healthcheck) performSingleHealthcheck(portsMapping map[int]int) (bool, error) {
	switch h.CheckType {
	case HttpGet200:
		res, err := http.Get(fmt.Sprintf("http://localhost:%d%s", portsMapping[h.Port], h.Data))
		if err != nil {
			return false, err
		}
		return res.StatusCode == 200, nil
	case Script:
		cmd := exec.Command("sh", "-c", h.Data)
		// Set the ports mapping env variables
		for k, v := range portsMapping {
			cmd.Env = append(cmd.Env, fmt.Sprintf("PORT%d=%d", k, v))
		}
		if err := cmd.Run(); err != nil {
			return false, err
		}
		return cmd.ProcessState.ExitCode() == 0, nil
	// TODO: Implement more healthchecks
	default:
		panic("unimplemented")
	}
}
