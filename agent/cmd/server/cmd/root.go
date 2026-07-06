package cmd

import (
	"github.com/2Panel-dev/2Panel/agent/server"
	"github.com/spf13/cobra"
)

var RootCmd = &cobra.Command{
	Use: "2panel-agent",
	RunE: func(cmd *cobra.Command, args []string) error {
		server.Start()
		return nil
	},
}
