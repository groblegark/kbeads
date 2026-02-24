package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/spf13/cobra"
)

var mailCmd = &cobra.Command{
	Use:                "mail",
	Short:              "Agent mail (delegates to bd mail)",
	Long:               `Delegates to 'bd mail' for agent-to-agent messaging. All arguments are forwarded.`,
	DisableFlagParsing: true,
	SilenceUsage:       true,
	RunE: func(cmd *cobra.Command, args []string) error {
		bd, err := exec.LookPath("bd")
		if err != nil {
			return fmt.Errorf("bd CLI not found in PATH — mail requires the beads daemon CLI")
		}
		argv := append([]string{"bd", "mail"}, args...)
		return syscall.Exec(bd, argv, os.Environ())
	},
}

var inboxCmd = &cobra.Command{
	Use:                "inbox",
	Short:              "Agent inbox (delegates to bd inbox)",
	Long:               `Delegates to 'bd inbox' for agent notifications. All arguments are forwarded.`,
	DisableFlagParsing: true,
	SilenceUsage:       true,
	RunE: func(cmd *cobra.Command, args []string) error {
		bd, err := exec.LookPath("bd")
		if err != nil {
			return fmt.Errorf("bd CLI not found in PATH — inbox requires the beads daemon CLI")
		}
		argv := append([]string{"bd", "inbox"}, args...)
		return syscall.Exec(bd, argv, os.Environ())
	},
}
