package cmd

import (
	"io"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
)

var verbosity int

var rootCmd = &cobra.Command{
	Use:   "biscepter",
	Short: "Efficient Git Bisect using Docker Caching for Fast Repeated and Concurrent Bisection",
	Long:  ``,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Set standard logger's verbosity
		if verbosity < 0 {
			logrus.SetOutput(io.Discard)
		} else if verbosity == 0 {
			logrus.SetLevel(logrus.WarnLevel)
		} else if verbosity == 1 {
			logrus.SetLevel(logrus.InfoLevel)
		} else if verbosity == 2 {
			logrus.SetLevel(logrus.DebugLevel)
		} else {
			logrus.SetLevel(logrus.TraceLevel)
		}
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Hide the completion command
	rootCmd.CompletionOptions.HiddenDefaultCmd = true

	// Init the logger
	formatter := prefixed.TextFormatter{
		TimestampFormat: "15:04:05.000",
		FullTimestamp:   true,
		ForceFormatting: true,
	}
	formatter.SetColorScheme(&prefixed.ColorScheme{
		TimestampStyle: "245",
	})
	logrus.SetFormatter(&formatter)

	rootCmd.PersistentFlags().IntVarP(&verbosity, "verbose", "v", 1, "Set the verbosity [0-3]")
}
