package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/hokaccha/go-prettyjson"
	"github.com/parnurzeal/gorequest"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "updatehub-ctl",
	Short: "UpdateHub Control Utility",
}

func main() {
	var probeServerAddress string

	probeCmd := &cobra.Command{
		Use:   "probe",
		Short: "Probe the server for update",
		RunE: func(cmd *cobra.Command, args []string) error {
			return execProbeCmd(probeServerAddress)
		},
	}

	infoCmd := &cobra.Command{
		Use:   "info",
		Short: "Print general information",
		RunE: func(cmd *cobra.Command, args []string) error {
			return execInfoCmd()
		},
	}

	logsCmd := &cobra.Command{
		Use:   "logs",
		Short: "Print agent log entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			return execLogsCmd()
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

func execProbeCmd(serverAddress string) error {
	var probe ProbeResponse

	var req struct {
		ServerAddress string `json:"server-address"`
	}
	req.ServerAddress = serverAddress

	_, _, errs := gorequest.New().Post(buildURL("/probe")).Send(req).EndStruct(&probe)
	if len(errs) > 0 {
		return errs[0]
	}

	output, _ := prettyjson.Marshal(probe)
	fmt.Println(string(output))

	return nil
}

func execInfoCmd() error {
	var info AgentInfo

	_, _, errs := gorequest.New().Get(buildURL("/info")).EndStruct(&info)
	if len(errs) > 0 {
		return errs[0]
	}

	output, _ := prettyjson.Marshal(info)
	fmt.Println(string(output))

	return nil
}

func execLogsCmd() error {
	_, body, errs := gorequest.New().Get(buildURL("/log")).End()
	if len(errs) > 0 {
		return errs[0]
	}

	var entries []LogEntry

	err := json.Unmarshal([]byte(body), &entries)
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
