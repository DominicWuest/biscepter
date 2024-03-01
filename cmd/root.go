package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var verbosity int

var rootCmd = &cobra.Command{
	Use:   "biscepter",
	Short: "Efficient Git Bisect using Docker Caching for Fast Repeated and Concurrent Bisection",
	Long:  ``,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().IntVarP(&verbosity, "verbose", "v", 1, "Set the verbosity [0-3]")
}
