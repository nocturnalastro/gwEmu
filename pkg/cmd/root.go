// SPDX-License-Identifier: GPL-2.0-or-later

package cmd

import (
	"fmt"
	"gwEmu/pkg/config"
	"gwEmu/pkg/resource"
	"gwEmu/pkg/transformers"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/printers"
)

func transformInput(filePath string) error {
	resources := resource.ParseResourceFile(filePath)
	log.Debug().Msgf("Found %d resources\n", len(resources))
	transformed, err := transformers.Transform(resources)
	if err != nil {
		return fmt.Errorf("failed to transform resources: %w", err)
	}
	log.Debug().Msgf("Recived %d transformed resources\n", len(transformed))
	y := &printers.YAMLPrinter{}
	for _, t := range transformed {
		fmt.Println("---")
		y.PrintObj(t, os.Stdout)
	}
	return nil
}

func printAndExit(msg any) {
	fmt.Println(msg)
	os.Exit(1)
}

var (
	filePath     string
	loglevel     string
	prefixSuffix string

	// rootCmd represents the base command when called without any subcommands
	rootCmd = &cobra.Command{
		Use:   "gwEmu",
		Short: "Generic Workload Emulator",
		Long:  `Generic Workload Emulator: A tool for turning a foot print into an emulated workload`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			level, err := zerolog.ParseLevel(loglevel)
			if err != nil {
				return err
			}
			zerolog.SetGlobalLevel(level)

			if prefixSuffix != "" {
				config.SetConfig("prefix-suffix", prefixSuffix)
			}

			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			if filePath == "" {
				printAndExit("no file path provided")
				return
			}
			stat, err := os.Stat(filePath)
			if err != nil {
				printAndExit(fmt.Errorf("failed to stat file: %w", err))
				return
			}
			if stat.IsDir() {
				printAndExit(fmt.Errorf("path to dir not file"))
				return
			}
			err = transformInput(filePath)
			if err != nil {
				printAndExit(err)
			}
		},
	}
)

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringVarP(
		&filePath,
		"file",
		"f",
		"",
		"path to input file",
	)
	rootCmd.PersistentFlags().StringVarP(
		&loglevel,
		"verbosity",
		"v",
		zerolog.LevelErrorValue,
		"path to input file ",
	)

	rootCmd.Flags().StringVarP(
		&prefixSuffix,
		"prefix-suffix",
		"p",
		"",
		"Extra suffix for label prefix",
	)

}
