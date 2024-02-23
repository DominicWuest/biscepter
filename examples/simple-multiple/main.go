package main

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/DominicWuest/biscepter/pkg/biscepter"
)

func main() {
	// Reading in the job config
	file, err := os.Open("jobConf.yml")
	if err != nil {
		panic(err)
	}

	job, err := biscepter.GetJobFromConfig(file)
	if err != nil {
		panic(err)
	}

	// Setting the replicas count
	job.ReplicasCount = 3

	// Running the job
	rsChan, ocChan, err := job.Run()
	if err != nil {
		panic(err)
	}

	// Waiting for running systems or offending commit
	offendingCommits := 0
	for {
		select {
		// Offending commit found
		case commit := <-ocChan:
			fmt.Printf("%d: Bisection done for replica with index %d! Offending commit: %s\n", commit.ReplicaIndex, commit.ReplicaIndex, commit.Commit)
			offendingCommits++
			if offendingCommits == 3 {
				fmt.Println("Finished bisecting all issues!")
				return
			}
		// New system to test online
		case system := <-rsChan:
			fmt.Printf("%d: Got running system on port %d for replica with index %d\n", system.ReplicaIndex, system.Ports[3333], system.ReplicaIndex)

			res, err := http.Get(fmt.Sprintf("http://localhost:%d/%d", system.Ports[3333], system.ReplicaIndex))
			if err != nil {
				panic(err)
			}

			resBytes, err := io.ReadAll(res.Body)
			if err != nil {
				panic(err)
			}
			resText := string(resBytes)

			fmt.Printf("%d: Got response %s\n", system.ReplicaIndex, resText)

			if resText == fmt.Sprint(system.ReplicaIndex) {
				fmt.Printf("%d: This commit is good!\n", system.ReplicaIndex)
				system.IsGood()
			} else {
				fmt.Printf("%d: This commit is bad!\n", system.ReplicaIndex)
				system.IsBad()
			}
		}
	}
}
