package main

import (
	"os"

	"github.com/spf13/cobra"
)

type zombieRecoveryCommand struct {
	cmd *cobra.Command
}

func newZombieRecoveryCommand() *cobra.Command {
	cc := &zombieRecoveryCommand{}
	cc.cmd = &cobra.Command{
		Use:   "zombierecovery",
		Short: "Try rescuing funds stuck in channels with zombie nodes",
		Long: `A sub command that hosts a set of further sub commands
to help with recovering funds tuck in zombie channels.

Please visit https://github.com/lightninglabs/chantools/blob/master/doc/zombierecovery.md
for more information on how to use these commands.`,
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				_ = cmd.Help()
				os.Exit(0)
			}
		},
	}

	cobra.EnableCommandSorting = false
	cc.cmd.AddCommand(
		// Here the order matters, we don't want them to be
		// alphabetically sorted but by step number.
		newZombieRecoveryFindMatchesCommand(),
		newZombieRecoveryPrepareKeysCommand(),
		newZombieRecoveryMakeOfferCommand(),
		newZombieRecoverySignOfferCommand(),
	)

	return cc.cmd
}
