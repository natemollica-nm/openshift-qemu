package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var Version = "1.6.0"

// getVersion composes the parts of the version in a way that's suitable
// for displaying to humans.
func getVersion() string {
	version := Version
	version = fmt.Sprintf("v%s", version)

	// Strip off any single quotes added by the git information.
	return strings.Replace(version, "'", "", -1)
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show openshift-qemu version info",
	Long:  `All software has versions. This is openshift-qemu's'`,
	Run: func(cmd *cobra.Command, args []string) {
		version := getVersion()
		fmt.Println(version)
	},
}
