package cli

import (
	"github.com/fedusia/crawler/internals/crawler"
	"github.com/spf13/cobra"
)

var limit int
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Start crawler",
	Run: func(cmd *cobra.Command, args []string) {
		crawler.Run(limit)
	},
}

func init() {
	runCmd.PersistentFlags().IntVar(&limit, "limit", -1, "Limit domains to check")
}
