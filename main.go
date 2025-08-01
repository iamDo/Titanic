package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	bubbletea "github.com/charmbracelet/bubbletea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/viper"
)


// getRemoteMap uses SSH to run md5sum on remote files

func getRemoteMap(source string) (map[string]string, error) {
	parts := strings.SplitN(source, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid remote source %s", source)
	}
	host, path := parts[0], parts[1]
	path = strings.TrimRight(path, "/")
	cmd := exec.Command("ssh", host, fmt.Sprintf("cd %s && find . -type f -exec md5sum {} +", path))
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ssh error: %w", err)
	}

	m := make(map[string]string)
	s := bufio.NewScanner(&out)
	for s.Scan() {
		fields := strings.Fields(s.Text())
		if len(fields) < 2 {
			continue
		}
		hash := fields[0]
		file := fields[1]
		rel := strings.TrimPrefix(file, "./")
		m[rel] = hash
	}
	return m, nil
}

// listLocalMap uses find+md5sum locally
func listLocalMap(dir string) (map[string]string, error) {
	dir = strings.TrimRight(dir, "/")
	cmd := exec.Command("sh", "-c", fmt.Sprintf("cd %s && find . -type f -exec md5sum {} +", dir))
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("local find error: %w", err)
	}

	m := make(map[string]string)
	s := bufio.NewScanner(&out)
	for s.Scan() {
		fields := strings.Fields(s.Text())
		if len(fields) < 2 {
			continue
		}
		hash := fields[0]
		file := fields[1]
		rel := strings.TrimPrefix(file, "./")
		m[rel] = hash
	}
	return m, nil
}

// syncDifferences syncs missing or mismatched files from source to destination
func syncDifferences(pair DirectoryPair) {
	srcMap := make(map[string]string)
	var err error
	if strings.Contains(pair.Source, ":") {
		srcMap, err = getRemoteMap(pair.Source)
	} else {
		srcMap, err = listLocalMap(pair.Source)
	}
	if err != nil {
		fmt.Println("Error refreshing source maps:", err)
		return
	}
	
	dstMap, err := listLocalMap(pair.Destination)
	if err != nil {
		fmt.Println("Error refreshing destination maps:", err)
		return
	}
	// sync each file
	for rel, sh := range srcMap {
		dh, ok := dstMap[rel]
		if ok && sh == dh {
			continue
		}
		// ensure destination dir exists
		destPath := filepath.Join(pair.Destination, rel)
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			fmt.Println("Error creating dest dir:", err)
			continue
		}
		// build rsync command
		var cmd *exec.Cmd
		if strings.Contains(pair.Source, ":") {
			hostPath := fmt.Sprintf("%s:%s/%s", strings.SplitN(pair.Source, ":", 2)[0], strings.TrimRight(strings.SplitN(pair.Source, ":", 2)[1], "/"), rel)
			cmd = exec.Command("rsync", "-avz", hostPath, destPath)
		} else {
			srcPath := filepath.Join(pair.Source, rel)
			cmd = exec.Command("rsync", "-avz", srcPath, destPath)
		}
		fmt.Println("Syncing", rel)
		if out, err := cmd.CombinedOutput(); err != nil {
			fmt.Printf("Error syncing %s: %v\nOutput: %s\n", rel, err, string(out))
		}
	}
}

// padColumns formats four columns: status, filename, srcHash, dstHash
func padColumns(status, file, src, dst string) string {
	return fmt.Sprintf("%-20s %-45s %-33s %-33s\n", status, file, src, dst)
}

// highlightDifferences compares source vs destination maps
func highlightDifferences(pair DirectoryPair) string {
	var srcMap map[string]string
	var err error
	if strings.Contains(pair.Source, ":") {
		srcMap, err = getRemoteMap(pair.Source)
	} else {
		srcMap, err = listLocalMap(pair.Source)
	}
	if err != nil {
		return fmt.Sprintf("Error retrieving source: %v", err)
	}
	// always local for destination
	dstMap, err := listLocalMap(pair.Destination)
	if err != nil {
		return fmt.Sprintf("Error retrieving destination: %v", err)
	}
	
	out := ""
	for rel, sh := range srcMap {
		dh, ok := dstMap[rel]
		if !ok {
			out += padColumns("Missing destination", rel, sh, "")
		} else if sh != dh {
			out += padColumns("Mismatch", rel, sh, dh)
		} else {
			out += padColumns("Match", rel, sh, dh)
		}
	}
	for rel, dh := range dstMap {
		if _, ok := srcMap[rel]; !ok {
			out += padColumns("Missing source", rel, "", dh)
		}
	}
	return out
}

func loadConfig() (Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")
	if err := viper.ReadInConfig(); err!=nil { return Config{}, err }
	var cfg Config
	if err := viper.Unmarshal(&cfg); err!=nil { return Config{}, err }
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
			diffs := highlightDifferences(pair)
			if diffs == "" {
				fmt.Println("All files in sync")
			} else {
				fmt.Print(diffs)
			}
		}
		return
	}
	p := bubbletea.NewProgram(NewModel(cfg), tea.WithAltScreen())
	if _, err := p.Run(); err!=nil { fmt.Println("run:",err) }
}