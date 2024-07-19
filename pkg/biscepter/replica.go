package biscepter

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"

	"github.com/dchest/uniuri"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/go-connections/nat"
	"github.com/otiai10/copy"
	"github.com/phayes/freeport"
	"github.com/sirupsen/logrus"
)

// A replica is a single instance of a job and is used to bisect one issue
type replica struct {
	parentJob *Job // The job from which this replica stems

	index int // Index of this replica in the list of all replicas

	id string // ID of this replica

	repoPath string // The path to this replica's copy of the repo under test

	goodCommitOffset int // The offset to the original bad commit of the newest good commit
	badCommitOffset  int // The offset to the original bad commit of the oldest bad commit

	commits []string // This replica's commits, where commits[0] is the good commit and commits[N-1] is the bad commit

	waitingCond *sync.Cond // Condition variable used by goroutine created in replica.start to wait until the current commit was reported to be good or bad

	isStopped bool // Whether this replica is running

	lastRunningSystem *RunningSystem // The last running system created by this replica. Is shut down when the replica is stopped

	log *logrus.Entry

	possibleOtherCommits []string
}

func createJobReplica(j *Job, index int, id string) (*replica, error) {
	// Copy the repo
	dir, err := os.MkdirTemp("", "")
	if err != nil {
		return nil, err
	}
	if err := copy.Copy(j.repoPath, dir, copy.Options{
		Specials:     true,
		NumOfWorkers: int64(j.MaxConcurrentReplicas),
	}); err != nil {
		return nil, err
	}

	return &replica{
		parentJob: j,

		index: index,
		id:    id,

		repoPath: dir,

		goodCommitOffset: 0,
		badCommitOffset:  len(j.commits) - 1,

		commits: j.commits,

		waitingCond: sync.NewCond(&sync.Mutex{}),

		log: j.Log.WithField("replica-id", id),
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
				r.log.Panicf("Replica %d failed to init next system - %v", r.index, err)
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
	if rs.commitRootOffset < r.goodCommitOffset {
		return
	}
	r.goodCommitOffset = rs.commitRootOffset

	// Release the in initNextSystem acquired semaphore with a weight of 1
	r.parentJob.replicaSemaphore.Release(1)

	go func() {
		if err := rs.stop(); err != nil {
			r.log.Warnf("Failed to stop container %s - %v", rs.containerName, err)
		}
	}()

	// Signal goroutine started in start() to wake up again
	r.waitingCond.L.Lock()
	r.waitingCond.Signal()
	r.waitingCond.L.Unlock()
}

func (r *replica) isBad(rs RunningSystem) {
	if rs.commitRootOffset > r.badCommitOffset {
		return
	}
	r.badCommitOffset = rs.commitRootOffset

	// Release the in initNextSystem acquired semaphore with a weight of 1
	r.parentJob.replicaSemaphore.Release(1)

	go func() {
		if err := rs.stop(); err != nil {
			r.log.Warnf("Failed to stop container %s - %v", rs.containerName, err)
		}
	}()

	// Signal goroutine started in start() to wake up again
	r.waitingCond.L.Lock()
	r.waitingCond.Signal()
	r.waitingCond.L.Unlock()
}

func (r *replica) initNextSystem() (*RunningSystem, error) {
	// Acquire the semaphore with a weight of 1
	r.parentJob.replicaSemaphore.Acquire(context.Background(), 1)

	nextCommit := r.getNextCommit()
	commitHash := getActualCommit(r.commits[nextCommit], r.parentJob.commitReplacements)

	// Checkout new commit
	cmd := exec.Command("sh", "-c", fmt.Sprintf("git add . && git reset --hard %s", commitHash))
	cmd.Dir = r.repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, errors.Join(fmt.Errorf("git checkout of hash %s at %s failed for replica %d, output: %s", commitHash, r.repoPath, r.index, out), err)
	}

	// Update all submodules
	cmd = exec.Command("git", "submodule", "update", "--init", "--recursive")
	cmd.Dir = r.repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, errors.Join(fmt.Errorf("git submodule update at %s failed for replica %d, output: %s", r.repoPath, r.index, out), err)
	}

	// Create docker client
	apiClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, errors.Join(fmt.Errorf("docker client creation failed for replica %d", r.index), err)
	}
	defer apiClient.Close()

	// Build the new image if it doesn't exist yet
	imageName := r.parentJob.getDockerImageOfCommit(commitHash)
	newLock := &sync.Mutex{}
	l, _ := r.parentJob.imagesBuilding.LoadOrStore(commitHash, newLock)
	lock := l.(*sync.Mutex)
	lock.Lock()
	if !r.parentJob.builtImages[imageName] {
		r.log.Infof("Building image %s of commit %s", imageName, commitHash)
		// Image has not been built yet
		// TODO: Have to ensure there is no dockerfile being overwritten in dest repo
		os.WriteFile(path.Join(r.repoPath, "Dockerfile"), []byte(r.parentJob.dockerfileString), 0777)
		ctx, err := archive.TarWithOptions(r.repoPath, &archive.TarOptions{})
		if err != nil {
			return nil, errors.Join(fmt.Errorf("tar creation of dockerfile for commit hash %s failed for replica %d", commitHash, r.index), err)
		}
		buildRes, err := apiClient.ImageBuild(context.Background(), ctx, types.ImageBuildOptions{
			Tags:        []string{imageName},
			ForceRemove: true,
			Labels:      map[string]string{"biscepter": "1"},
		})
		if err != nil {
			out, _ := io.ReadAll(buildRes.Body)
			logrus.Warnf("Image build of %s for commit hash %s failed, avoiding commit from now on. Build output: %s", imageName, commitHash, out)
			r.parentJob.builtImages[imageName] = true
			r.replaceCommit(nextCommit)
			lock.Unlock()
			return r.initNextSystem()
		}
		// Wait for build to be done
		out, err := io.ReadAll(buildRes.Body)
		if err != nil {
			return nil, err
		}
		logrus.Tracef("Image build output:\n%s", string(out))

		// Check if last stream message is an error-detail, meaning the build failed
		strOut := strings.Split(string(out[:len(out)-1]), "\n")
		if strings.HasPrefix(strOut[len(strOut)-1], `{"errorDetail"`) {
			r.log.Warnf("Image build of %s for commit hash %s failed, avoiding commit from now on. Build output: %s", imageName, commitHash, out)
			r.replaceCommit(nextCommit)
			// Set to true s.t. waiting replicas don't attempt to rebuild
			r.parentJob.builtImages[imageName] = true
			lock.Unlock()
			return r.initNextSystem()
		}
		r.parentJob.builtImages[imageName] = true
		lock.Unlock()
	} else {
		if _, ok := r.parentJob.commitReplacements.Load(commitHash); ok {
			// Commit breaks the build, init another system
			r.log.Warnf("Image for commit hash %s reported to be broken, reattempting to init next system.", commitHash)
			lock.Unlock()
			return r.initNextSystem()
		}
		// Image has been built - reuse it
		r.log.Infof("Image %s of commit %s already built, reusing image", imageName, commitHash)
		lock.Unlock()
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
		Labels:       map[string]string{"biscepter": "1"},
	}

	// Setup the host config
	hostConfig := &container.HostConfig{
		AutoRemove:   true,
		PortBindings: portBindings,
	}

	containerName := "biscepter-" + uniuri.New()

	r.log.Debugf("Exposed ports: %+v, Port bindings: %+v", exposedPorts, portBindings)

	// Create the new container
	resp, err := apiClient.ContainerCreate(context.Background(), containerConfig, hostConfig, nil, nil, containerName)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("container creation with name %s of image %s failed for replica %d", containerName, imageName, r.index), err)
	}

	// Start the new container
	if err := apiClient.ContainerStart(context.Background(), resp.ID, container.StartOptions{}); err != nil {
		return nil, errors.Join(fmt.Errorf("container start with name %s and id %s of image %s failed for replica %d", containerName, resp.ID, imageName, r.index), err)
	}

	r.log.Infof("Started container %s running commit %s, performing healthchecks...", containerName, commitHash)

	// Perform healthchecks
	for _, healthcheck := range r.parentJob.Healthchecks {
		success, err := healthcheck.performHealthcheck(ports, r.log)
		if !success {
			return nil, fmt.Errorf("healthcheck on port %d failed for replica %d", healthcheck.Port, r.index)
		} else if err != nil {
			return nil, err
		}
	}

	r.log.Infof("Successfully performed healthchecks on container %s running commit %s", containerName, commitHash)

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
	nextCommit := (r.goodCommitOffset + r.badCommitOffset) / 2

	// Find closest cached build
	offset := 0
	for i := 0; i < r.badCommitOffset-nextCommit; i++ {
		commitAbove := r.parentJob.getDockerImageOfCommit(r.commits[nextCommit+i])
		commitBelow := r.parentJob.getDockerImageOfCommit(r.commits[nextCommit-i])

		if r.parentJob.builtImages[commitAbove] {
			// If a commit above the middle is built
			offset = i
			break
		} else if r.parentJob.builtImages[commitBelow] && nextCommit-i > r.goodCommitOffset {
			// If a commit below the middle is built. Since nextCommit rounds down, we have to check we're not testing the same commit again
			offset = -i
			break
		}
	}
	// Check, based on buildCost, whether this offset is worth it
	/*
		Definitions:
		bC := build cost
		c := % of cached builds between current good and bad commit
		E[o] := expected number of runs if the offset is chosen (log_2(E[number of commits if offset is chosen]))
		E[nO] := expected number of runs if the offset is not chosen (log_2(commits left to check))

		oCost := Cost when using the offset
		nOCost := Cost when not using the offset

		oCost = E[o] * c + E[o] * (1 - c) * bC + 1 // +1 since we know the next commit would be cached
		nOCost = E[nO] * c + E[nO] * (1 - c) * bC + bC // +bC since we know the next commit has to be built

		Use offset, if oCost < nOCost, else don't use the offset
	*/
	offsetCommit := nextCommit + offset
	if offset != 0 {
		commitsLeft := r.badCommitOffset - r.goodCommitOffset - 2
		commitsAbove := r.badCommitOffset - offsetCommit - 1
		commitsBelow := offsetCommit - r.goodCommitOffset - 1

		// Assuming uniform distribution of buggy commits, calculate probabilities that commit is above/below
		chanceAbove := float64(commitsAbove) / float64(commitsLeft)
		chanceBelow := float64(commitsBelow) / float64(commitsLeft)

		// Remaining commits estimate if we use this offset
		expectedCommits := chanceAbove*float64(commitsAbove) + chanceBelow*float64(commitsBelow)

		expectedRuns := math.Log2(expectedCommits)         // Expected runs with using cached build
		expectedRunsOld := math.Log2(float64(commitsLeft)) // Expected runs without using cached build

		// Get the fraction of cached vs uncached commits
		cached := 0
		for i := r.goodCommitOffset + 1; i < r.badCommitOffset-1; i++ {
			if r.parentJob.builtImages[r.parentJob.getDockerImageOfCommit(r.commits[i])] {
				cached++
			}
		}

		cachedFraction := float64(cached) / float64(commitsLeft)
		uncachedFraction := 1.0 - cachedFraction

		offsetCost := cachedFraction*expectedRuns + uncachedFraction*expectedRuns*r.parentJob.BuildCost + 1
		noOffsetCost := cachedFraction*expectedRunsOld + uncachedFraction*(expectedRunsOld+1)*r.parentJob.BuildCost

		r.log.Debugf("Expected commits left if we use offset %d: %f. Commits left: Total: %d, Above: %d, Below: %d. Chance of bug being in commit Above: %f, Below: %f. Expected Runs: %f, Expected Runs without Offset: %f. Cached Fraction: %f, Offset Cost: %f, No Offset Cost: %f.", offset, expectedCommits, commitsLeft, commitsAbove, commitsBelow, chanceAbove, chanceBelow, expectedRuns, expectedRunsOld, cachedFraction, offsetCost, noOffsetCost)

		// Make sure that we're actually saving time by reusing this build
		if noOffsetCost < offsetCost {
			r.log.Debugf("Would not save time by running offset %d", offset)
			offset = 0
		} else {
			r.log.Debugf("Time saved by running offset %d", offset)
		}
	}

	r.log.Debugf("Good commit %d, Bad commit %d, middle commit %d, next commit %d (offset %d)", r.goodCommitOffset, r.badCommitOffset, nextCommit, offsetCommit, offset)
	r.log.Infof("Expected amount of runs left: ~%.1f", math.Log2(float64(r.badCommitOffset-r.goodCommitOffset)))
	return nextCommit + offset
}

// getOffendingCommit returns the offending commit for the issue bisected by the replica if it was found.
// If no offending commit was yet found, returns nil
func (r *replica) getOffendingCommit() *OffendingCommit {
	// Offending commit not yet found
	if r.badCommitOffset > r.goodCommitOffset+1 {
		return nil
	}

	commitHash := getActualCommit(r.commits[r.badCommitOffset], r.parentJob.commitReplacements)
	prevCommitHash := getActualCommit(r.commits[r.badCommitOffset-1], r.parentJob.commitReplacements)

	// Get all commits that alias to the current one
	curCommit := commitHash
	for {
		found := false
		r.parentJob.commitReplacements.Range(
			func(key, value any) bool {
				commit := key.(string)
				replacement := value.(string)
				if replacement == curCommit {
					curCommit = commit
					r.possibleOtherCommits = append(r.possibleOtherCommits, commit)
					found = true
					return false
				}
				return true
			},
		)
		if !found {
			break
		}
	}

	// TODO: Maybe toggle this off with a flag? Or specify a max depth of bisecting merges? Also document that octopus merges are not supported.
	// TODO: Check if commit is a merge commit but no octopus commit, bisect merge branch if yes

	mergeParent, err := getMergedParent(commitHash, prevCommitHash, r.repoPath)
	if err != nil {
		r.log.Errorf("Failed to get merge parent of %s - %v", commitHash, err)
	}

	if mergeParent != "" {
		r.log.Infof("Offending commit %s is a merge commit. Merged parent: %s", commitHash, mergeParent)
		var err error
		r.commits, err = getCommitsBetween(prevCommitHash, mergeParent, r.repoPath)
		if err != nil {
			r.log.Panicf("couldn't get replica's merge commits - %v", err)
		}
		r.goodCommitOffset = 0
		r.badCommitOffset = len(r.commits) - 1
		return nil
	}

	// Get additional info about the commit
	var commitMsg, commitDate, commitAuthor string
	cmd := exec.Command("git", "--no-pager", "show", "-s", "--format=%B%n%aD%n%an <%ae>", getActualCommit(commitHash, r.parentJob.commitReplacements))
	cmd.Dir = r.repoPath
	outBytes, err := cmd.CombinedOutput()
	if err != nil {
		r.log.Errorf("Couldn't get additional offending commit info - %v, output: %s", err, outBytes)
	} else {
		out := string(outBytes)
		if len(out) == 0 || strings.Count(out, "\n") < 3 {
			r.log.Warnf("Git show output is not of the expected format: %q", out)
		} else {
			// Trim trailing newline
			out = out[:len(out)-1]
			authorOffset := strings.LastIndex(out, "\n")
			dateOffset := strings.LastIndex(out[:authorOffset], "\n")

			commitMsg = strings.TrimSpace(out[:dateOffset])
			commitDate = out[dateOffset+1 : authorOffset]
			commitAuthor = out[authorOffset+1:]
		}
	}

	r.log.Infof("Found offending commit %s with offset %d. Message: %q, Date: %q, Author: %q", commitHash, r.badCommitOffset, commitMsg, commitDate, commitAuthor)

	return &OffendingCommit{
		ReplicaIndex: r.index,

		Commit:       commitHash,
		CommitOffset: r.badCommitOffset,

		CommitMessage: commitMsg,
		CommitDate:    commitDate,
		CommitAuthor:  commitAuthor,

		PossibleOtherCommits: r.possibleOtherCommits,
	}
}

// replaceCommit makes note of the passed commit as breaking the build.
// Once the function returns, a replacement commit will have been set in this job's replacementCommit map for the passed commit.
//
// Since it is assumed that the ends of the commits slice are commits that build, as they otherwise couldn't have been evaluated, this function panics if
//
//	commitOffset >= len(commits) - 1
func (r *replica) replaceCommit(commitOffset int) {
	if commitOffset >= len(r.commits)-1 {
		logrus.Panicf("Passed commit offset %d to replaceCommit is too large! Max allowed length :%d", commitOffset, len(r.commits)-2)
	}

	// Get the offset of the actual commit to replace
	cur := r.commits[commitOffset]
	for {
		if val, ok := r.parentJob.commitReplacements.Load(cur); ok {
			cur = val.(string)
			commitOffset++
		} else {
			break
		}
	}

	next := r.commits[commitOffset+1]

	// Store in replacements file for reuse in later runs
	r.parentJob.commitReplacementsBackupFile.WriteString(fmt.Sprintf("%s:%s,", cur, next))

	r.log.Debugf("Adding new replacement: %s -> %s", cur, next)

	r.parentJob.commitReplacements.Store(cur, next)
}

// A RunningSystem is a running system that is ready to be tested
type RunningSystem struct {
	ReplicaIndex int // The index of this system's parent replica

	Ports map[int]int // A mapping of the ports specified for the system under test to the ones they were mapped to locally

	parentReplica *replica

	containerName string // The name of the container running this system

	commit           string // The current commit
	commitRootOffset int    // The offset of the current commit to the root commit

	wasRated bool // If this system was already specified to be either good or bad
}

// IsGood tells biscepter that this running system is good.
// If IsGood is called after the running system was already rated by a previous IsGood or IsBad method invocation, it will panic.
func (r *RunningSystem) IsGood() {
	if r.wasRated {
		panic(fmt.Sprintf("IsGood was called on running system of replica with index %d after it was already rated", r.ReplicaIndex))
	}
	r.wasRated = true
	r.parentReplica.isGood(*r)
}

// IsBad tells biscepter that this running system is bad.
// If IsBad is called after the running system was already rated by a previous IsGood or IsBad method invocation, it will panic.
func (r *RunningSystem) IsBad() {
	if r.wasRated {
		panic(fmt.Sprintf("IsBad was called on running system of replica with index %d after it was already rated", r.ReplicaIndex))
	}
	r.wasRated = true
	r.parentReplica.isBad(*r)
}

func (r RunningSystem) stop() error {
	// Create docker client
	apiClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	defer apiClient.Close()

	if err := apiClient.ContainerStop(context.Background(), r.containerName, container.StopOptions{}); err != nil {
		return err
	}
	return nil
}

// An OffendingCommit represents the finished bisection of a replica.
type OffendingCommit struct {
	ReplicaIndex int // The index of the bisected replica

	Commit       string // The commit which introduced the issue. I.e. the oldest bad commit
	CommitOffset int    // The offset to the initial commit of the commit which introduced the issue. I.e. the offset of the oldest bad commit

	CommitMessage string // The message of the offending commit
	CommitDate    string // The date of the offending commit
	CommitAuthor  string // The author of the offending commit

	PossibleOtherCommits []string // Other possible offending commits. Set if there were build failures causing uncertainty in the exact offending commit
}
