// Command lsm (Lsm iS Manager) is a cross-platform manager for long-running
// background services described by a YAML configuration file.
package main

import (
	"os"

	"github.com/LiZeC123/lsm/cmd"
)

func main() {
	os.Exit(cmd.Execute())
}
