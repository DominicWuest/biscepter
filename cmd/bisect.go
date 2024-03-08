package cmd

import (
	"os"
	"strconv"

	"github.com/DominicWuest/biscepter/internal/server"
	"github.com/DominicWuest/biscepter/pkg/biscepter"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var bisectPort int
var bisectConcurrency uint

var bisectCmd = &cobra.Command{
	Use:   "bisect job.yml [replicas]",
	Short: "Start a server for bisecting an issue based on a job.yml",
	Long: `Start a server for bisecting an issue based on a job.yml.
This command optionally takes in an additional value for the amount of replicas should be launched.
If no value for this is specified, it defaults to one replica.

Calling this command results in a RESTful HTTP server being created, with whose API the issue(s) can be bisected.`,
	Args: cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		jobYaml, err := os.Open(args[0])
		if err != nil {
			logrus.Fatalf("Failed to open job yaml - %v", err)
		}
		job, err := biscepter.GetJobFromConfig(jobYaml)
		if err != nil {
			logrus.Fatalf("Failed to read job config from yaml - %v", err)
		}

		replicas := 1
		if len(args) == 2 {
			var err error
			replicas, err = strconv.Atoi(args[1])
			if err != nil {
				logrus.Fatalf("%s not a valid argument for amount of replicas", args[1])
			}
		}
		job.ReplicasCount = replicas
		job.Log = logrus.StandardLogger()
		job.MaxConcurrentReplicas = bisectConcurrency

		rsChan, ocChan, err := job.Run()
		if err != nil {
			logrus.Fatalf("Failed to start job - %v", err)
		}

		serverType := server.HTTP
		err = server.NewServer(serverType, bisectPort, rsChan, ocChan)
		if err != nil {
			logrus.Fatalf("Failed to start webserver - %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(bisectCmd)

	bisectCmd.Flags().IntVarP(&bisectPort, "port", "p", 40032, "The port on which to start the server")
	bisectCmd.Flags().UintVarP(&bisectConcurrency, "max-concurrency", "c", 0, "The max amount of replicas that can run concurrently, or 0 if no limit")
}
