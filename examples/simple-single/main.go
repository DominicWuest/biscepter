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
	job.ReplicasCount = 1

	// Running the job
	rsChan, ocChan, err := job.Run()
	if err != nil {
		panic(err)
	}

	// Waiting for running systems or the offending commit
	for {
		select {
		// Offending commit found
		case commit := <-ocChan:
			fmt.Printf("Bisection done! Offending commit: %s\nCommit message: %s\n", commit.Commit, commit.CommitMessage)

			if err := job.Stop(); err != nil {
				panic(err)
			}
			return
		// New system to test online
		case system := <-rsChan:
			fmt.Printf("Got running system on port %d\n", system.Ports[3333])

			res, err := http.Get(fmt.Sprintf("http://localhost:%d/1", system.Ports[3333]))
			if err != nil {
				panic(err)
			}

			resBytes, err := io.ReadAll(res.Body)
			if err != nil {
				panic(err)
			}
			resText := string(resBytes)

			fmt.Printf("Got response %s\n", resText)

			if resText == "1" {
				fmt.Println("This commit is good!")
				system.IsGood()
			} else {
				fmt.Println("This commit is bad!")
				system.IsBad()
			}
		}
	}
}
