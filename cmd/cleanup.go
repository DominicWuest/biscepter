package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/manifoldco/promptui"
	"github.com/moby/moby/client"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var cleanupContainers bool
var cleanupAgree bool

var cleanupCmd = &cobra.Command{
	Use:     "clean",
	Aliases: []string{"prune", "cleanup"},
	Short:   "Clean all docker artifacts created by biscepter",
	Long: `This command cleans all docker artifacts by biscepter.
This includes containers, both running and stopped, as well as all docker images built.`,
	Run: func(cmd *cobra.Command, args []string) {
		cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			logrus.Fatalf("Couldn't create docker client - %v", err)
		}
		defer cli.Close()

		containers, err := cli.ContainerList(context.Background(), container.ListOptions{
			All: true,
			Filters: filters.NewArgs(
				filters.KeyValuePair{
					Key:   "label",
					Value: "biscepter=1",
				},
			),
		})
		if err != nil {
			logrus.Fatalf("Couldn't list docker containers - %v", err)
		}

		images, err := cli.ImageList(context.Background(), image.ListOptions{
			All: true,
			Filters: filters.NewArgs(
				filters.KeyValuePair{
					Key:   "label",
					Value: "biscepter=1",
				},
			),
		})
		if err != nil {
			logrus.Fatalf("Couldn't list docker images - %v", err)
		}

		if cleanupContainers {
			images = []image.Summary{}
		}

		if len(containers)+len(images) == 0 {
			imageString := " or images"
			if cleanupContainers {
				imageString = ""
			}
			logrus.Infof("No containers%s to remove. Exiting...", imageString)
			return
		}

		confirmationMessage := fmt.Sprintf("About to delete %d containers", len(containers))
		if !cleanupContainers {
			confirmationMessage += fmt.Sprintf(" and %d images", len(images))
		}
		confirmationMessage += "."
		logrus.Info(confirmationMessage)

		prompt := promptui.Prompt{
			Label:     "Proceed",
			IsConfirm: true,
		}

		if !cleanupAgree {
			_, err := prompt.Run()
			if err != nil {
				logrus.Info("Exiting...")
				os.Exit(0)
			}
		}

		for _, c := range containers {
			logrus.Infof("Deleting container %s (ID: %s)", c.Names[0][1:], c.ID)
			if err := cli.ContainerRemove(context.Background(), c.ID, container.RemoveOptions{Force: true}); err != nil {
				logrus.Fatalf("Failed to remove container with ID %s - %v", c.ID, err)
			}
		}

		for _, i := range images {
			logrus.Infof("Deleting image %s (ID: %s)", i.RepoTags[0], i.ID)
			if _, err := cli.ImageRemove(context.Background(), i.ID, image.RemoveOptions{
				PruneChildren: true,
				Force:         true,
			}); err != nil {
				logrus.Fatalf("Failed to remove image with ID %s - %v", i.ID, err)
			}
		}

		logrus.Info("Done cleaning up.")
	},
}

func init() {
	rootCmd.AddCommand(cleanupCmd)

	cleanupCmd.Flags().BoolVarP(&cleanupContainers, "containers", "c", false, "Only delete containers, no images.")
	cleanupCmd.Flags().BoolVarP(&cleanupAgree, "assume-yes", "y", false, `Bypass "Are you sure?" message.`)
}
