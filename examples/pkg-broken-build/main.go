package main

import (
	"os"

	"github.com/DominicWuest/biscepter/pkg/biscepter"
	"github.com/sirupsen/logrus"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
)

func main() {
	// Set logging format
	formatter := prefixed.TextFormatter{
		DisableTimestamp: true,
	}
	formatter.SetColorScheme(&prefixed.ColorScheme{})
	logrus.SetFormatter(&formatter)

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
	job.ReplicasCount = 2

	// Set logging output for biscepter
	job.Log = logrus.StandardLogger()
	job.Log.SetLevel(logrus.WarnLevel)

	// Running the job
	rsChan, ocChan, err := job.Run()
	if err != nil {
		panic(err)
	}

	// Waiting for running systems or the offending commit
	offendingCommits := 0
	for {
		select {
		// Offending commit found
		case oc := <-ocChan:
			logrus.SetLevel(logrus.InfoLevel)
			logrus.Printf("Bisection done!")
			logrus.SetLevel(logrus.WarnLevel)
			offendingCommits++
			if offendingCommits == 2 {
				logrus.SetLevel(logrus.InfoLevel)
				logrus.Printf("Bisection avoided broken commit and reports the bad commit as being %q.", oc.Commit)
				logrus.Printf("Commit message: %q", oc.CommitMessage)
				if err := job.Stop(); err != nil {
					panic(err)
				}
				return
			}
		// New system to test online
		case system := <-rsChan:
			// Just assume it is good for demo purposes
			system.IsGood()
		}
	}
}
