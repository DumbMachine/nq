package cmd

import (
	"fmt"
	"os"

	"github.com/dumbmachine/nq"
	"github.com/spf13/cobra"
)

// TODO: This should have ( cancel, restart, status )
// taskCmd represents the task command
var queueCMD = &cobra.Command{
	Use:   "queue <subcommand> [flags]",
	Short: "Manage queue",
	Long:  `Manage queue lifecycle using cli`,
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: show help message for queue
		if len(args) == 0 {
			cmd.Help()
			os.Exit(0)
		}
	},
}

var queueStatsCmd = &cobra.Command{
	Use:     "stats --name=<name> [flags]",
	Aliases: []string{"s"},
	Short:   "Stats",
	Long:    (`Basic statistics for a queue`),
	Example: (`$ nq queue stats --name=<queue>`),
	Run: func(cmd *cobra.Command, args []string) {
		qname, err := cmd.Flags().GetString("name")
		if err != nil || qname == "" {
			if len(args) == 0 {
				cmd.Help()
				os.Exit(0)
			}
			fmt.Println("invalid value for required argument name")
			os.Exit(1)
		}

		uri, err := cmd.Flags().GetString("uri")
		if err != nil || uri == "" {
			fmt.Println("invalid value for required argument uri")
			os.Exit(1)
		}
		client := nq.NewPublishClient(nq.NatsClientOpt{Addr: uri}, nq.NoAuthentcation())
		if err := client.Stats(qname); err != nil {
			fmt.Printf("%s\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(queueCMD)
	queueCMD.AddCommand(queueStatsCmd)
	queueStatsCmd.Flags().StringP("name", "n", "", "name of queue")
}
