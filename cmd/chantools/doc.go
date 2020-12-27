package main

import (
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

func newDocCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use: "doc",
		Short: "Generate the markdown documentation of all commands",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return doc.GenMarkdownTree(rootCmd, "./doc")
		},
	}

	return cmd
}