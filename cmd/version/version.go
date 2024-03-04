package version

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version holds the version binary built with - must be injected from the build process via -ldflags="-X 'github.com/plumber-cd/argocd-cmp-replicator/cmd/version.Version=dev'"
var Version = "dev"

// versionCmd will print the version
var Cmd = &cobra.Command{
	Use:   "version",
	Short: "Print version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(Version)
	},
}
