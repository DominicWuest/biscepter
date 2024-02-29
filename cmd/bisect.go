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

var (
	bisectHTTP      bool
	bisectWebsocket bool
)

var bisectCmd = &cobra.Command{
	Use:   "bisect job.yml [replicas]",
	Short: "Start a server for bisecting an issue based on a job.yml",
	Long: `Start a server for bisecting an issue based on a job.yml.
This command optionally takes in an additional value for the amount of replicas should be launched.
If no value for this is specified, it defaults to one replica.

Calling this command results in a server being created, with whose API the issue(s) can be bisected.
By default, this will be an HTTP server, but other options also exist.
The options include:
	- HTTP
	- Websocket`,
	Args: cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		jobYaml, err := os.Open(args[0])
		if err != nil {
			logrus.Errorf("Failed to open job yaml - %v", err)
			logrus.Exit(1)
		}
		job, err := biscepter.GetJobFromConfig(jobYaml)
		if err != nil {
			logrus.Errorf("Failed to read job config from yaml - %v", err)
			logrus.Exit(1)
		}

		replicas := 1
		if len(args) == 2 {
			var err error
			replicas, err = strconv.Atoi(args[1])
			if err != nil {
				logrus.Errorf("%s not a valid argument for amount of replicas", args[1])
				logrus.Exit(1)
			}
		}
		job.ReplicasCount = replicas

		rsChan, ocChan, err := job.Run()
		if err != nil {
			logrus.Errorf("Failed to start job - %v", err)
			logrus.Exit(1)
		}

		serverType := server.HTTP
		if bisectWebsocket {
			serverType = server.Websocket
		}
		err = server.NewServer(serverType, bisectPort, rsChan, ocChan)
		if err != nil {
			logrus.Errorf("Failed to start webserver - %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(bisectCmd)

	bisectCmd.Flags().IntVarP(&bisectPort, "port", "p", 40032, "The port on which to start the server")

	bisectCmd.Flags().BoolVar(&bisectHTTP, "http-server", false, "Start an HTTP webserver for handling API requests [default]")
	bisectCmd.Flags().BoolVar(&bisectWebsocket, "websocket-server", false, "Start a websocket webserver for handling API requests")
	bisectCmd.MarkFlagsMutuallyExclusive("http-server", "websocket-server")

}
