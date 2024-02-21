package biscepter

import (
	"io"

	"gopkg.in/yaml.v3"
)

type jobConfig struct {
	Repository string `yaml:"repository"`

	GoodCommit string `yaml:"goodCommit"`
	BadCommit  string `yaml:"badCommit"`

	ErrorExitCode int `yaml:"errorExitCode"`

	Port  int   `yaml:"port"`
	Ports []int `yaml:"ports"`

	Healthcheck []healthcheckConf `yaml:"healthcheck"`

	Dockerfile             string `yaml:"dockerfile"`
	DockerfilePath         string `yaml:"dockerfilePath"`
	DockerfilePathRelative string `yaml:"dockerfilePathRelative"`

	BuildCost float64 `yaml:"buildCost"`
}

// GetJobFromConfig reads in a job config in yaml format from a reader and initializes the corresponding job struct
func GetJobFromConfig(r io.Reader) (Job, error) {
	var config jobConfig

	decoder := yaml.NewDecoder(r)
	if err := decoder.Decode(config); err != nil {
		return Job{}, err
	}

	return Job{}, nil
}

// A job represents a blueprint for replicas, which are then used to bisect one issue.
// Jobs can create multiple replicas at once.
type Job struct {
	ReplicasCount int // How many replicas of itself this job should spawn simultaneously. Each replica is to be used for bisecting one issue.

	BuildCost float64 // TODO: Explain this precisely

	Ports        []int         // The ports which this job needs
	Healthchecks []Healthcheck // The healthchecks for this job

	Dockerfile             string // The contents of the dockerfile.
	DockerfilePath         string // The path to the dockerfile relative to the present working directory. Only gets used if Dockerfile is empty.
	DockerfilePathRelative string // The path to the dockerfile relative to the project's root. Only gets used if Dockerfile and DockerfilePath are empty.

	replicas []*replica // This job's replicas
}

// Run the job. This initializes all the replicas and starts them. This function returns a [RunningSystem] channel and an [OffendingCommit] channel.
// The [RunningSystem] channel should be used to get notified about systems which are ready to be tested.
// Once an [OffendingCommit] was received for a given replica index, no more [RunningSystem] structs for this replica will appear in the [RunningSystem] channel.
func (j *Job) Run() (chan RunningSystem, chan OffendingCommit, error) {
	// TODO: Create replicas, initiate channel for ready replicas, run replicas
	panic("unimplemented")
}
