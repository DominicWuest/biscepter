package cmd

import (
	"io"
	"os"
	"strconv"

	"github.com/DominicWuest/biscepter/internal/server"
	"github.com/DominicWuest/biscepter/pkg/biscepter"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var bisectPort int

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
		job.Log = logrus.New()

		// Set logger verbosity
		if verbosity < 0 {
			job.Log.SetOutput(io.Discard)
		} else if verbosity == 0 {
			job.Log.SetLevel(logrus.WarnLevel)
		} else if verbosity == 1 {
			job.Log.SetLevel(logrus.InfoLevel)
		} else if verbosity == 2 {
			job.Log.SetLevel(logrus.DebugLevel)
		} else {
			job.Log.SetLevel(logrus.TraceLevel)
		}

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

	bisectCmd.MarkFlagsMutuallyExclusive("http-server", "websocket-server")

}
