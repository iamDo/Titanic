package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	bubbletea "github.com/charmbracelet/bubbletea"
	tea      "github.com/charmbracelet/bubbletea"
	"github.com/spf13/viper"
	"titanic_app/diff"
)

// getFileHashes returns local or remote file hashes via diff package
func getFileHashes(src string) ([]diff.FileHash, error) {
	if strings.Contains(src, ":") {
		return diff.ListRemote(src)
	}
	return diff.ListLocal(src)
}

// highlightDifferences computes diffs and formats them
func highlightDifferences(pair DirectoryPair) string {
	srcList, err := getFileHashes(pair.Source)
	if err != nil {
		return fmt.Sprintf("Error retrieving source: %v\n", err)
	}
	dstList, err := diff.ListLocal(pair.Destination)
	if err != nil {
		return fmt.Sprintf("Error retrieving destination: %v\n", err)
	}
	diffs := diff.ComputeDiff(srcList, dstList)
	var out strings.Builder
	for _, d := range diffs {
		var status string
		switch d.Status {
		case diff.Match:
			status = "Match"
		case diff.MissingSource:
			status = "Missing source"
		case diff.MissingDestination:
			status = "Missing destination"
		case diff.Mismatch:
			status = "Mismatch"
		}
		out.WriteString(fmt.Sprintf("%-20s %-45s %-33s %-33s\n", status, d.Path, d.SrcHash, d.DstHash))
	}
	return out.String()
}

// loadConfig reads config from ./config/config.yaml
func loadConfig() (Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")
	if err := viper.ReadInConfig(); err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func main() {
	noTUI := flag.Bool("no-tui", false, "output without TUI")
	flag.Parse()

	cfg, err := loadConfig()
	if err != nil {
		fmt.Println("config:", err)
		os.Exit(1)
	}

	if *noTUI {
		for i, pair := range cfg.DirectoryPairs {
			fmt.Printf("Pair %d/%d %s -> %s\n", i+1, len(cfg.DirectoryPairs), pair.Source, pair.Destination)
			body := highlightDifferences(pair)
			if body == "" {
				fmt.Println("All files in sync")
			} else {
				fmt.Print(body)
			}
		}
		return
	}

	p := bubbletea.NewProgram(NewModel(cfg), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Println("run:", err)
		os.Exit(1)
	}
}
