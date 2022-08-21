package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "nq <command> <subcommand> [flags]",
	Short: "nq",
	Long:  `Command line tool to inspect tasks and queues managed by nq`,
	Annotations: map[string]string{
		"help:feedback": `Open an issue at https://github.com/dumbmachine/nq/issues/new/choose`,
	},
}

var versionOutput = fmt.Sprintf("nq version %s\n", "0.1.0")

var versionCmd = &cobra.Command{
	Use:    "version",
	Hidden: false,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Print(versionOutput)
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {

	cobra.OnInitialize(func() {
		// todo: store nats uri in a config file
	})

	rootCmd.AddCommand(versionCmd)
	rootCmd.SetVersionTemplate(versionOutput)

	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.PersistentFlags().StringP("uri", "u", "", "uri to nats")
}
