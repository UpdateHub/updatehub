package main

import (
	"fmt"
	"log"

	"github.com/hokaccha/go-prettyjson"
	"github.com/spf13/cobra"
	"github.com/UpdateHub/agent-sdk-go"
)

var rootCmd = &cobra.Command{
	Use:   "updatehub-ctl",
	Short: "UpdateHub Control Utility",
}

func main() {
	var probeServerAddress string

	agent := updatehub.NewClient()

	probeCmd := &cobra.Command{
		Use:   "probe",
		Short: "Probe the server for update",
		RunE: func(cmd *cobra.Command, args []string) error {
			return execProbeCmd(agent, probeServerAddress)
		},
	}

	infoCmd := &cobra.Command{
		Use:   "info",
		Short: "Print general information",
		RunE: func(cmd *cobra.Command, args []string) error {
			return execInfoCmd(agent)
		},
	}

	logsCmd := &cobra.Command{
		Use:   "logs",
		Short: "Print agent log entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			return execLogsCmd(agent)
		},
	}

	probeCmd.Flags().StringVarP(&probeServerAddress, "server-address", "s", "", "Server address for the triggered probe")

	rootCmd.AddCommand(probeCmd)
	rootCmd.AddCommand(infoCmd)
	rootCmd.AddCommand(logsCmd)

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func execProbeCmd(agent *updatehub.Client, serverAddress string) error {
	probe, err := agent.ProbeCustomServer(serverAddress)
	if err != nil {
		return err
	}

	output, _ := prettyjson.Marshal(probe)
	fmt.Println(string(output))

	return nil
}

func execInfoCmd(agent *updatehub.Client) error {
	info, err := agent.GetInfo()
	if err != nil {
		return err
	}

	output, _ := prettyjson.Marshal(info)
	fmt.Println(string(output))

	return nil
}

func execLogsCmd(agent *updatehub.Client) error {
	entries, err := agent.GetLogs()
	if err != nil {
		return err
	}

	for _, entry := range entries {
		output, _ := prettyjson.Marshal(entry)
		fmt.Println(string(output))
	}

	return nil
}

func buildURL(path string) string {
	return fmt.Sprintf("http://localhost:8080/%s", path[1:])
}
