package cmd

import (
	"fmt"
	"os"

	"github.com/dumbmachine/nq"
	"github.com/spf13/cobra"
)

// TODO: This should have ( cancel, restart, status )
// taskCmd represents the task command
var taskCmd = &cobra.Command{
	Use:     "task <subcommand> [flags]",
	Aliases: []string{"t"},
	Short:   "Manage tasks",
	Long:    `Manage task lifecycle using cli`,
	Example: `  nq task status --id=<id>
  nq task cancel --id=<id>`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			cmd.Help()
			os.Exit(0)
		}
	},
}

var taskStatusCmd = &cobra.Command{
	Use:     "status --id=<id>",
	Aliases: []string{"s"},
	Short:   "Task status",
	Long:    (`Get status of an nq task`),
	Example: (`  nq task status --id=<id>`),
	Run: func(cmd *cobra.Command, args []string) {
		id, err := cmd.Flags().GetString("id")
		if err != nil || id == "" {
			if len(args) == 0 {
				cmd.Help()
				os.Exit(0)
			}
			fmt.Println("invalid value for required argument id")
			os.Exit(1)
		}

		uri, err := cmd.Flags().GetString("uri")
		if err != nil || uri == "" {
			fmt.Println("invalid value for required argument uri")
			os.Exit(1)
		}
		client := nq.NewPublishClient(nq.NatsClientOpt{Addr: uri}, nq.NoAuthentcation())
		if msg, err := client.Fetch(id); err != nil {
			fmt.Printf("%s", err)
			os.Exit(1)
		} else {
			fmt.Printf("taskID=%s status=%s", msg.ID, msg.GetStatus())
			os.Exit(0)
		}
	},
}

var taskCancelCmd = &cobra.Command{
	Use:     "cancel --id=<id>",
	Aliases: []string{"c"},
	Short:   "Task cancel",
	Long:    (`Cancel an nq task`),
	Example: (`  nq task cancel --id=<id>`),
	Run: func(cmd *cobra.Command, args []string) {
		id, err := cmd.Flags().GetString("id")
		if err != nil || id == "" {
			if len(args) == 0 {
				cmd.Help()
				os.Exit(0)
			}
			fmt.Println("invalid value for required argument id")
			os.Exit(1)
		}

		uri, err := cmd.Flags().GetString("uri")
		if err != nil || uri == "" {
			fmt.Println("invalid value for required argument uri")
			os.Exit(1)
		}
		client := nq.NewPublishClient(nq.NatsClientOpt{Addr: uri}, nq.NoAuthentcation())
		if err := client.Cancel(id); err != nil {
			fmt.Printf("%s\n", err)
			os.Exit(1)
		} else {
			fmt.Printf("Cancel message sent task=%s", id)
			os.Exit(0)
		}
	},
}

func init() {
	rootCmd.AddCommand(taskCmd)
	taskCmd.Flags().BoolP("help", "h", false, "Get this help message")

	taskCmd.AddCommand(taskStatusCmd)
	taskStatusCmd.Flags().StringP("id", "i", "", "task id")

	taskCmd.AddCommand(taskCancelCmd)
	taskCancelCmd.Flags().StringP("id", "i", "", "task id")
}
