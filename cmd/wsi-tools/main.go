package main

import (
	"fmt"
	"os"

	_ "github.com/cornish/wsi-tools/internal/codec/all"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "wsi-tools",
	Short: "Utilities for whole-slide imaging (WSI) files",
	Long: `wsi-tools — a Swiss-army knife for whole-slide imaging files used in digital pathology.

Run 'wsi-tools <command> --help' for command-specific flags and examples.`,
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
