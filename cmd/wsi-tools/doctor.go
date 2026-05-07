package main

import (
	"fmt"

	codec "github.com/cornish/wsi-tools/internal/codec"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Report installed codec libraries + version info",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("wsi-tools", Version, "— codec / library health check.")
		fmt.Println()
		fmt.Println("Registered codecs:")
		for _, name := range codec.List() {
			fmt.Printf("  %s\n", name)
		}
		fmt.Println()
		fmt.Println("Required libs (probed at link time, not runtime):")
		fmt.Println("  libjpeg-turbo")
		fmt.Println("  libopenjp2")
		fmt.Println("  github.com/cornish/opentile-go")
		return nil
	},
}

func init() { rootCmd.AddCommand(doctorCmd) }
