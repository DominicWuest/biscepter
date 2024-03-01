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

	// This is just here to make the output prettier
	var colors map[int]string = map[int]string{
		0: "\u001B[0;35m",
		1: "\u001B[0;32m",
		2: "\u001B[0;36m",
	}
	colorReset := "\u001B[0m"

	// Waiting for running systems or offending commit
	offendingCommits := 0
	for {
		select {
		// Offending commit found
		case commit := <-ocChan:
			fmt.Printf("%s%d: Bisection done for replica with index %d! Offending commit: %s\nCommit message: %s%s\n", colors[commit.ReplicaIndex], commit.ReplicaIndex, commit.ReplicaIndex, commit.Commit, commit.CommitMessage, colorReset)
			offendingCommits++
			if offendingCommits == 3 {
				fmt.Println("Finished bisecting all issues!")

				if err := job.Stop(); err != nil {
					panic(err)
				}
				return
			}
		// New system to test online
		case system := <-rsChan:
			fmt.Printf("%s%d: Got running system on port %d for replica with index %d%s\n", colors[system.ReplicaIndex], system.ReplicaIndex, system.Ports[3333], system.ReplicaIndex, colorReset)

			res, err := http.Get(fmt.Sprintf("http://localhost:%d/%d", system.Ports[3333], system.ReplicaIndex+3))
			if err != nil {
				panic(err)
			}

			resBytes, err := io.ReadAll(res.Body)
			if err != nil {
				panic(err)
			}
			resText := string(resBytes)

			fmt.Printf("%s%d: Got response %s%s\n", colors[system.ReplicaIndex], system.ReplicaIndex, resText, colorReset)

			if resText == fmt.Sprint(system.ReplicaIndex+3) {
				fmt.Printf("%s%d: This commit is good!%s\n", colors[system.ReplicaIndex], system.ReplicaIndex, colorReset)
				system.IsGood()
			} else {
				fmt.Printf("%s%d: This commit is bad!%s\n", colors[system.ReplicaIndex], system.ReplicaIndex, colorReset)
				system.IsBad()
			}
		}
	}
}
