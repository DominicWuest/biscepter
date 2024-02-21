package biscepter

// A replica is a single instance of a job and is used to bisect one issue
type replica struct {
	parentJob *Job // The job from which this replica stems

	index int // Index of this replica in the list of all replicas

	goodCommitOffset int // The offset to the root of the newest good commit
	badCommitOffset  int // The offset to the root of the oldest bad commit
}

func createJobReplica(j Job) replica {
	panic("unimplemented")
}

func (r *replica) start() {
	panic("unimplemented")
}

func (r *replica) isGood(rs RunningSystem) {
	panic("unimplemented")
}

func (r *replica) isBad(rs RunningSystem) {
	panic("unimplemented")
}

// A RunningSystem is a running system that is ready to be tested
type RunningSystem struct {
	ReplicaIndex int // The index of this system's parent replica

	Ports map[int]int // A mapping of the ports specified for the system under test to the ones they were mapped to locally

	parentReplica *replica

	commit     string // The current commit
	rootOffset int    // The offset of the current commit to the root commit
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
