package biscepter

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	_ "crypto/sha1"

	"github.com/creasty/defaults"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/semaphore"
	"gopkg.in/yaml.v3"
)

type jobYaml struct {
	Repository string `yaml:"repository"`

	GoodCommit string `yaml:"goodCommit"`
	BadCommit  string `yaml:"badCommit"`

	Port  int   `yaml:"port"`
	Ports []int `yaml:"ports"`

	Healthcheck []healthcheckYaml `yaml:"healthcheck"`

	Dockerfile     string `yaml:"dockerfile"`
	DockerfilePath string `yaml:"dockerfilePath"`

	BuildCost float64 `yaml:"buildCost"`
}

// GetJobFromConfig reads in a job config in yaml format from a reader and initializes the corresponding job struct
func GetJobFromConfig(r io.Reader) (*Job, error) {
	var config jobYaml

	// Read in yaml
	decoder := yaml.NewDecoder(r)
	if err := decoder.Decode(&config); err != nil {
		return nil, err
	}

	// Convert to Job struct
	job := Job{
		BuildCost: config.BuildCost,

		GoodCommit: config.GoodCommit,
		BadCommit:  config.BadCommit,

		Dockerfile:     config.Dockerfile,
		DockerfilePath: config.DockerfilePath,

		Repository: config.Repository,
	}

	job.Ports = config.Ports
	if config.Port != 0 {
		job.Ports = []int{config.Port}
	}

	// Set all the healthchecks
	checkTypes := map[string]HealthcheckType{
		"http":   HttpGet200,
		"script": Script,
	}
	for _, check := range config.Healthcheck {
		if err := defaults.Set(&check); err != nil {
			return nil, err
		}
		checkType, ok := checkTypes[strings.ToLower(check.Type)]
		if !ok {
			return nil, fmt.Errorf("invalid check type supplied for healthcheck %s", check.Type)
		}

		job.Healthchecks = append(job.Healthchecks, Healthcheck{
			Port:      check.Port,
			CheckType: checkType,

			Data: check.Data,
			Config: HealthcheckConfig{
				Retries: check.Retries,

				Backoff: check.Backoff * time.Millisecond,

				BackoffIncrement: check.BackoffIncrement * time.Millisecond,
				MaxBackoff:       check.MaxBackoff * time.Millisecond,
			},
		})
	}

	return &job, nil
}

// A job represents a blueprint for replicas, which are then used to bisect one issue.
// Jobs can create multiple replicas at once.
type Job struct {
	ReplicasCount int // How many replicas of itself this job should spawn simultaneously. Each replica is to be used for bisecting one issue.

	// The cost multiplier of building a commit compared to running an already built commit.
	// A build cost of 100 means building a commit is 100 times more expensive than running a built commit.
	// A build cost of less than 1 results in biscepter always building the middle commit (if it was not built yet) and not using nearby, cached, builds.
	BuildCost float64

	Ports        []int         // The ports which this job needs
	Healthchecks []Healthcheck // The healthchecks for this job

	GoodCommit string // The hash of the good commit, i.e. the commit which does not exhibit any issues
	BadCommit  string // The hash of the bad commit, i.e. the commit which exhibits the issue(s) to be bisected

	Dockerfile     string // The contents of the dockerfile.
	DockerfilePath string // The path to the dockerfile relative to the present working directory. Only gets used if Dockerfile is empty.

	Log *logrus.Logger // The log to which information gets printed to

	MaxConcurrentReplicas uint // The max amount of replicas that can run concurrently, or 0 if no limit
	replicaSemaphore      *semaphore.Weighted

	dockerfileString string // The parsed dockerfile for building the repository
	dockerfileHash   string // The hash of the dockerfile string, for differentiating them in built images

	replicas []*replica // This job's replicas

	Repository string // The repository URL
	repoPath   string // The path to the original cloned repository which replicas will copy from

	commits []string // This job's commits, where commits[0] is the good commit and commits[N-1] is the bad commit

	builtImages map[string]bool // A hashmap where, if a commit exists as a key, this commit's docker image has already been built before

	imagesBuilding *sync.Map // Map of keys for every commit to ensure only one replica is building a specific commit at once

	commitReplacements *sync.Map // Map of commits to the commits they should be replaced with. used to avoid commits that break the build

	// Path to the file where commit replacements are written to and stored for subsequent runs. Defaults to "$(PWD)/.biscepter-replacements~"
	CommitReplacementsBackup     string
	commitReplacementsBackupFile *os.File
}

// Run the job. This initializes all the replicas and starts them. This function returns a [RunningSystem] channel and an [OffendingCommit] channel.
// The [RunningSystem] channel should be used to get notified about systems which are ready to be tested.
// Once an [OffendingCommit] was received for a given replica index, no more [RunningSystem] structs for this replica will appear in the [RunningSystem] channel.
func (job *Job) Run() (chan RunningSystem, chan OffendingCommit, error) {
	// Init the logger
	if job.Log == nil {
		// Mute logger
		job.Log = logrus.New()
		job.Log.SetOutput(io.Discard)
	}

	// Init the replica semaphore
	if job.MaxConcurrentReplicas == 0 {
		job.MaxConcurrentReplicas = math.MaxInt
	}
	job.replicaSemaphore = semaphore.NewWeighted(int64(job.MaxConcurrentReplicas))

	// Init the sync maps
	job.imagesBuilding = &sync.Map{}
	job.commitReplacements = &sync.Map{}

	// Read in the stored replacements
	if job.CommitReplacementsBackup == "" {
		job.CommitReplacementsBackup = ".biscepter-replacements~"
	}
	var err error
	job.commitReplacementsBackupFile, err = os.OpenFile(".biscepter-replacements~", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return nil, nil, errors.Join(fmt.Errorf("couldn't get replacements backup"), err)
	}
	replacements, err := os.ReadFile(".biscepter-replacements~")
	if err != nil {
		return nil, nil, errors.Join(fmt.Errorf("couldn't read replacements"), err)
	}

	replacementPairs := strings.Split(strings.TrimSuffix(string(replacements), ","), ",")
	if replacementPairs[0] != "" {
		for _, pair := range replacementPairs {
			split := strings.Split(pair, ":")
			if len(split) != 2 {
				return nil, nil, fmt.Errorf("format of replacements file entry incorrect: %s", pair)
			}
			job.Log.Debugf("Adding replacement from replacements file: %s -> %s", split[0], split[1])
			job.commitReplacements.Store(split[0], split[1])
		}
	}

	// Populate job.dockerfileBytes, depending on which values were present in the config
	if err := job.parseDockerfile(); err != nil {
		return nil, nil, err
	}

	job.Log.Info("Cloning initial repository...")
	// Clone repo
	job.repoPath, err = os.MkdirTemp("", "")
	if err != nil {
		return nil, nil, err
	}
	if out, err := exec.Command("git", "clone", job.Repository, job.repoPath).CombinedOutput(); err != nil {
		return nil, nil, errors.Join(fmt.Errorf("git clone of repository %s at %s failed, output: %s", job.Repository, job.repoPath, out), err)
	}

	job.Log.Info("Checking good and bad commits...")
	// Make sure there is a path from BadCommit to GoodCommit
	cmd := exec.Command("git", "rev-list", "--reverse", "--first-parent", job.BadCommit)
	cmd.Dir = job.repoPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, nil, errors.Join(fmt.Errorf("failed to get rev-list of bad commit %s, output: %s", job.BadCommit, out), err)
	}
	if !strings.Contains(string(out), job.GoodCommit) {
		return nil, nil, fmt.Errorf("good commit %s cannot be reached from bad commit %s", job.GoodCommit, job.BadCommit)
	}

	job.Log.Info("Getting all commits...")
	// Get all commits
	job.commits, err = getCommitsBetween(job.GoodCommit, job.BadCommit, job.repoPath)
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't get commits between %s and %s - %v", job.GoodCommit, job.BadCommit, err)
	}

	job.Log.Info("Getting all built images...")
	// Get all built images
	job.builtImages = make(map[string]bool)
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, nil, errors.Join(fmt.Errorf("failed to create new docker client"), err)
	}
	images, err := cli.ImageList(context.Background(), types.ImageListOptions{
		All: true,
		Filters: filters.NewArgs(
			filters.KeyValuePair{
				Key:   "label",
				Value: "biscepter=1",
			},
		),
	})
	if err != nil {
		return nil, nil, errors.Join(fmt.Errorf("failed to list all docker images"), err)
	}
	for _, image := range images {
		for _, tag := range image.RepoTags {
			logrus.Debugf("Adding new built repo tag: %s", tag)
			job.builtImages[tag] = true
		}
	}
	cli.Close()

	job.Log.Info("Creating replicas...")
	// Make the channels
	// TODO: Don't hardcode channel size
	rsChan, ocChan := make(chan RunningSystem, 100), make(chan OffendingCommit, 100)

	job.replicas = make([]*replica, job.ReplicasCount)

	// Create all replicas
	for i := range job.ReplicasCount {
		var err error
		// Create a new replica
		job.replicas[i], err = createJobReplica(job, i)
		if err != nil {
			// Stop running replicas
			for j := range i {
				if err := job.replicas[j].stop(); err != nil {
					return nil, nil, err
				}
			}
			return nil, nil, errors.Join(fmt.Errorf("failed to create job replica"), err)
		}

		// Start the created replica
		if err = job.replicas[i].start(rsChan, ocChan); err != nil {
			// Stop running replicas
			for j := range i {
				if err := job.replicas[j].stop(); err != nil {
					return nil, nil, errors.Join(fmt.Errorf("failed to stop job replica %d after start of %d failed", j, i), err)
				}
			}
			return nil, nil, errors.Join(fmt.Errorf("failed to start job replica %d", i), err)
		}
	}

	return rsChan, ocChan, nil
}

// Stop the job and all running replicas.
func (j *Job) Stop() error {
	for _, replica := range j.replicas {
		if err := replica.stop(); err != nil {
			return err
		}
	}

	return os.RemoveAll(j.repoPath)
}

// parseDockerfile sets j.dockerfileString based on the fields set.
// It prioritizes Dockerfile but uses DockerfilePath if it is empty.
// In addition, it sets dockerfileHash
func (j *Job) parseDockerfile() error {
	j.dockerfileString = j.Dockerfile
	if j.dockerfileString == "" {
		file, err := os.ReadFile(j.DockerfilePath)
		if err != nil {
			return err
		}
		j.dockerfileString = string(file)
	}
	j.dockerfileHash = digest.FromString(j.dockerfileString).Encoded()
	return nil
}

// getDockerImageOfCommit returns the name with the tag of the docker image which built the passed commit
func (j *Job) getDockerImageOfCommit(commit string) string {
	return fmt.Sprintf("biscepter-%s:%s", commit, j.dockerfileHash)
}
