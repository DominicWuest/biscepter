package biscepter

import (
	"os"
	"os/exec"
	"sync"

	"github.com/moby/moby/daemon/graphdriver/copy"
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

	// Clean up tmp directory of repo
	return os.RemoveAll(r.repoPath)
}

func (r *replica) isGood(rs RunningSystem) {
	// TODO: Panic if called twice for same commit
	if rs.commitRootOffset < r.goodCommitOffset {
		return
	}
	r.goodCommitOffset = rs.commitRootOffset + 1

	// Signal goroutine started in start() to wake up again
	r.waitingCond.L.Lock()
	r.waitingCond.Signal()
	r.waitingCond.L.Unlock()
}

func (r *replica) isBad(rs RunningSystem) {
	// TODO: Panic if called twice for same commit
	if rs.commitRootOffset > r.badCommitOffset {
		return
	}
	r.badCommitOffset = rs.commitRootOffset

	// Signal goroutine started in start() to wake up again
	r.waitingCond.L.Lock()
	r.waitingCond.Signal()
	r.waitingCond.L.Unlock()
}

func (r *replica) initNextSystem() (*RunningSystem, error) {
	nextCommit := r.getNextCommit()
	commitHash := r.parentJob.commits[nextCommit]

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

	return &RunningSystem{
		ReplicaIndex: r.index,

		// TODO: Ports

		parentReplica: r,

		commit:           commitHash,
		commitRootOffset: nextCommit,
	}, nil
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

	commit           string // The current commit
	commitRootOffset int    // The offset of the current commit to the root commit
}

func (r RunningSystem) IsGood() {
	r.parentReplica.isGood(r)
}

func (r RunningSystem) IsBad() {
	r.parentReplica.isBad(r)
}

// An OffendingCommit represents the finished bisection of a replica.
type OffendingCommit struct {
	ReplicaIndex int // The index of the bisected replica

	Commit       string // The commit which introduced the issue. I.e. the oldest bad commit
	CommitOffset int    // The offset to the initial commit of the commit which introduced the issue. I.e. the offset of the oldest bad commit
}
