package cli

import (
	apiv1 "github.com/acorn-io/acorn/pkg/apis/api.acorn.io/v1"
	cli "github.com/acorn-io/acorn/pkg/cli/builder"
	"github.com/acorn-io/acorn/pkg/cli/builder/table"
	"github.com/acorn-io/acorn/pkg/client"
	"github.com/acorn-io/acorn/pkg/system"
	"github.com/acorn-io/baaah/pkg/typed"
	"github.com/spf13/cobra"
)

func NewSecretExpose(c client.CommandContext) *cobra.Command {
	cmd := cli.Command(&Expose{client: c.ClientFactory}, cobra.Command{
		Use:     "expose [flags] [SECRET_NAME...]",
		Aliases: []string{"secrets", "s"},
		Example: `
acorn secret`,
		SilenceUsage: true,
		Short:        "Manage secrets",
		Args:         cobra.MinimumNArgs(1),
	})
	return cmd
}

type Expose struct {
	Quiet  bool   `usage:"Output only names" short:"q"`
	Output string `usage:"Output format (json, yaml, {{gotemplate}})" short:"o"`
	client client.ClientFactory
}

type exposeEntry struct {
	Name  string
	Type  string
	Key   string
	Value string
}

func (a *Expose) Run(cmd *cobra.Command, args []string) error {
	client, err := a.client.CreateDefault()
	if err != nil {
		return err
	}

	out := table.NewWriter([][]string{
		{"NAME", "Name"},
		{"TYPE", "Type"},
		{"KEY", "Key"},
		{"VALUE", "Value"},
	}, system.UserNamespace(), a.Quiet, a.Output)

	var matchedSecrets []apiv1.Secret

	for _, arg := range args {
		secret, err := client.SecretExpose(cmd.Context(), arg)
		if err != nil {
			return err
		}
		matchedSecrets = append(matchedSecrets, *secret)
	}

	for _, secret := range matchedSecrets {
		for _, entry := range typed.Sorted(secret.Data) {
			out.Write(&exposeEntry{
				Name:  secret.Name,
				Type:  secret.Type,
				Key:   entry.Key,
				Value: string(entry.Value),
			})
		}
	}

	return out.Err()
}
