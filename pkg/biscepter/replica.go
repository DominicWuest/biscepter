package biscepter

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"sync"

	"github.com/dchest/uniuri"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/go-connections/nat"
	"github.com/moby/moby/daemon/graphdriver/copy"
	"github.com/phayes/freeport"
)

// A replica is a single instance of a job and is used to bisect one issue
type replica struct {
	parentJob *Job // The job from which this replica stems

	index int // Index of this replica in the list of all replicas

	repoPath string // The path to this replica's copy of the repo under test

	goodCommitOffset int // The offset to the original bad commit of the newest good commit
	badCommitOffset  int // The offset to the original bad commit of the oldest bad commit

	waitingCond *sync.Cond // Condition variable used by goroutine created in replica.start to wait until the current commit was reported to be good or bad

	isStopped bool // Whether this replica is running

	lastRunningSystem *RunningSystem // The last running system created by this replica. Is shut down when the replica is stopped
}

func createJobReplica(j *Job, index int) (*replica, error) {
	// Copy the repo
	dir, err := os.MkdirTemp("", "")
	if err != nil {
		return nil, err
	}
	if err := copy.DirCopy(j.repoPath, dir, copy.Content, false); err != nil {
		return nil, err
	}

	return &replica{
		parentJob: j,

		index: index,

		repoPath: dir,

		goodCommitOffset: 0,
		badCommitOffset:  len(j.commits) - 1,

		waitingCond: sync.NewCond(&sync.Mutex{}),
	}, nil
}

func (r *replica) start(rsChan chan RunningSystem, ocChan chan OffendingCommit) error {
	// Create goroutine for the replica
	go func() {
		for !r.isStopped {
			// Check if offending commit was found, terminate if yes
			if oc := r.getOffendingCommit(); oc != nil {
				ocChan <- *oc
				break
			}
			r.waitingCond.L.Lock()

			readySystem, err := r.initNextSystem()
			if err != nil {
				// TODO: What to do here?
				panic(err)
			}
			rsChan <- *readySystem

			// Wait until commit was reported to be good or bad
			r.waitingCond.Wait()
			r.waitingCond.L.Unlock()
		}
	}()

	return nil
}

func (r *replica) stop() error {
	// Stop goroutine
	r.isStopped = true
	r.waitingCond.Signal()

	if r.lastRunningSystem != nil {
		r.lastRunningSystem.stop()
	}

	// Clean up tmp directory of repo
	return os.RemoveAll(r.repoPath)
}

func (r *replica) isGood(rs RunningSystem) {
	// TODO: Panic if called twice for same commit - Also document in rs.IsGood
	if rs.commitRootOffset < r.goodCommitOffset {
		return
	}
	r.goodCommitOffset = rs.commitRootOffset + 1

	go func() {
		if err := rs.stop(); err != nil {
			panic(err)
		}
	}()

	// Signal goroutine started in start() to wake up again
	r.waitingCond.L.Lock()
	r.waitingCond.Signal()
	r.waitingCond.L.Unlock()
}

func (r *replica) isBad(rs RunningSystem) {
	// TODO: Panic if called twice for same commit - Also document in rs.IsBad
	if rs.commitRootOffset > r.badCommitOffset {
		return
	}
	r.badCommitOffset = rs.commitRootOffset

	go func() {
		if err := rs.stop(); err != nil {
			panic(err)
		}
	}()

	// Signal goroutine started in start() to wake up again
	r.waitingCond.L.Lock()
	r.waitingCond.Signal()
	r.waitingCond.L.Unlock()
}

func (r *replica) initNextSystem() (*RunningSystem, error) {
	nextCommit := r.getNextCommit()
	commitHash := r.parentJob.commits[nextCommit]

	fmt.Println("Testing ", commitHash)

	// Checkout new commit
	cmd := exec.Command("git", "checkout", commitHash)
	cmd.Dir = r.repoPath
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	// Update all submodules
	cmd = exec.Command("git", "submodule", "update", "--init", "--recursive")
	cmd.Dir = r.repoPath
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	// Create docker client
	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}
	defer apiClient.Close()

	// Build the new image if it doesn't exist yet
	imageName := "biscepter-" + commitHash
	r.parentJob.imagesBuilding[commitHash].Lock()
	if !r.parentJob.builtImages[imageName] {
		// Image has not been built yet
		// TODO: Have to ensure there is no dockerfile being overwritten in dest repo
		os.WriteFile(path.Join(r.repoPath, "Dockerfile"), []byte(r.parentJob.dockerfileString), 0777)
		ctx, err := archive.TarWithOptions(r.repoPath, &archive.TarOptions{})
		if err != nil {
			return nil, err
		}
		buildRes, err := apiClient.ImageBuild(context.Background(), ctx, types.ImageBuildOptions{
			Tags:        []string{imageName},
			ForceRemove: true,
		})
		if err != nil {
			return nil, err
		}
		// Wait for build to be done
		_, err = io.ReadAll(buildRes.Body)
		if err != nil {
			return nil, err
		}
		r.parentJob.builtImages[imageName] = true
		r.parentJob.imagesBuilding[commitHash].Unlock()
	} else {
		// Image has been built - reuse it
		r.parentJob.imagesBuilding[commitHash].Unlock()
	}

	// Setup the ports
	ports := make(map[int]int)
	exposedPorts := make(nat.PortSet)
	portBindings := make(nat.PortMap)

	// Add all needed ports to the ports map
	for _, healthcheck := range r.parentJob.Healthchecks {
		ports[healthcheck.Port] = 0
	}
	for _, port := range r.parentJob.Ports {
		ports[port] = 0
	}

	// Assign free ports
	for port := range ports {
		natPort := nat.Port(fmt.Sprint(port))

		freePort, err := freeport.GetFreePort()
		if err != nil {
			return nil, err
		}

		exposedPorts[natPort] = struct{}{}
		portBindings[natPort] = []nat.PortBinding{{HostPort: fmt.Sprint(freePort)}}
		ports[port] = freePort
	}

	// Setup the container config
	containerConfig := &container.Config{
		Image:        imageName,
		ExposedPorts: exposedPorts,
	}

	// Setup the host config
	hostConfig := &container.HostConfig{
		AutoRemove:   true,
		PortBindings: portBindings,
	}

	containerName := "biscepter-" + uniuri.New()

	// Create the new container
	resp, err := apiClient.ContainerCreate(context.Background(), containerConfig, hostConfig, nil, nil, containerName)
	if err != nil {
		return nil, err
	}

	// Start the new container
	if err := apiClient.ContainerStart(context.Background(), resp.ID, container.StartOptions{}); err != nil {
		return nil, err
	}

	// Perform healthchecks
	for _, healthcheck := range r.parentJob.Healthchecks {
		success, err := healthcheck.performHealthcheck(ports)
		if !success {
			return nil, fmt.Errorf("healthcheck on port %d failed", healthcheck.Port)
		} else if err != nil {
			return nil, err
		}
	}

	rs := &RunningSystem{
		ReplicaIndex: r.index,

		Ports: ports,

		parentReplica: r,

		containerName: containerName,

		commit:           commitHash,
		commitRootOffset: nextCommit,
	}

	r.lastRunningSystem = rs

	return rs, nil
}

// getNextCommit returns the next commit which should be used for bisection
func (r replica) getNextCommit() int {
	return (r.goodCommitOffset + r.badCommitOffset) / 2
}

// getOffendingCommit returns the offending commit for the issue bisected by the replica if it was found.
// If no offending commit was yet found, returns nil
func (r *replica) getOffendingCommit() *OffendingCommit {
	// Offending commit not yet found
	if r.badCommitOffset != r.goodCommitOffset {
		return nil
	}

	// TODO: Check if commit is a merge commit, bisect merge branch if yes

	return &OffendingCommit{
		ReplicaIndex: r.index,

		Commit:       r.parentJob.commits[r.goodCommitOffset],
		CommitOffset: r.goodCommitOffset,
	}
}

// A RunningSystem is a running system that is ready to be tested
type RunningSystem struct {
	ReplicaIndex int // The index of this system's parent replica

	Ports map[int]int // A mapping of the ports specified for the system under test to the ones they were mapped to locally

	parentReplica *replica

	containerName string // The name of the container running this system

	commit           string // The current commit
	commitRootOffset int    // The offset of the current commit to the root commit
}

func (r RunningSystem) IsGood() {
	r.parentReplica.isGood(r)
}

func (r RunningSystem) IsBad() {
	r.parentReplica.isBad(r)
}

func (r RunningSystem) stop() error {
	// Create docker client
	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return err
	}
	defer apiClient.Close()

	if err := apiClient.ContainerStop(context.Background(), r.containerName, container.StopOptions{}); err != nil {
		// TODO: Probably just logging should be enough?
		return err
	}
	return nil
}

// An OffendingCommit represents the finished bisection of a replica.
type OffendingCommit struct {
	ReplicaIndex int // The index of the bisected replica

	Commit       string // The commit which introduced the issue. I.e. the oldest bad commit
	CommitOffset int    // The offset to the initial commit of the commit which introduced the issue. I.e. the offset of the oldest bad commit
}
