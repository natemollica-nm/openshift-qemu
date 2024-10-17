package main

import (
	"openshift-qemu/cmd" // Import the cmd package for the CLI
)

func main() {
	// This kicks off the Cobra root command execution
	cmd.Execute()
}
