package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/plexusone/omniagent/internal/version"
)

var (
	versionJSON bool
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Long:  "Display omniagent version, build information, and runtime details.",
	Run:   showVersion,
}

func init() {
	versionCmd.Flags().BoolVar(&versionJSON, "json", false, "output as JSON")
}

func showVersion(cmd *cobra.Command, args []string) {
	info := version.Get()

	if versionJSON {
		output, _ := json.MarshalIndent(info, "", "  ")
		fmt.Println(string(output))
		return
	}

	fmt.Println(info.String())
}
