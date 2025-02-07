package cli

import (
	"fmt"

	cli "github.com/acorn-io/acorn/pkg/cli/builder"
	"github.com/acorn-io/acorn/pkg/client"
	"github.com/spf13/cobra"
)

func NewContainerDelete(c client.CommandContext) *cobra.Command {
	cmd := cli.Command(&ContainerDelete{client: c.ClientFactory}, cobra.Command{
		Use: "kill [CONTAINER_NAME...]",
		Example: `
acorn container kill app-name.containername-generated-hash`,
		SilenceUsage: true,
		Short:        "Delete a container",
		Aliases:      []string{"rm"},
	})
	return cmd
}

type ContainerDelete struct {
	client client.ClientFactory
}

func (a *ContainerDelete) Run(cmd *cobra.Command, args []string) error {
	client, err := a.client.CreateDefault()
	if err != nil {
		return err
	}

	for _, container := range args {
		replicaDelete, err := client.ContainerReplicaDelete(cmd.Context(), container)
		if err != nil {
			return fmt.Errorf("deleting %s: %w", container, err)
		}
		if replicaDelete != nil {
			fmt.Println(container)
		} else {
			fmt.Printf("Error: No such container: %s\n", container)
		}
	}

	return nil
}
