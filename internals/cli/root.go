package cli

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
	Use:   "crawler",
	Short: "Crawler for tls detection on web sites",
	Long:  "Crawler goes through all ru sites and get information about tls version",
}

func Run() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(runCmd)
}
