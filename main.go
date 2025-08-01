package main

import (
	"path/filepath"
	"flag"
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/viper"
	bubbletea "github.com/charmbracelet/bubbletea"
)

// DirectoryPair represents a source (local or remote) and destination
// Remote source format: host:/absolute/path/
type DirectoryPair struct {
	Source      string `mapstructure:"source"`
	Destination string `mapstructure:"destination"`
}

// Config holds directory pairs
type Config struct {
	DirectoryPairs []DirectoryPair `mapstructure:"directory_pairs"`
}

// md5Hash computes MD5 for a local file
func md5Hash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	
	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// getRemoteMap uses SSH to run md5sum on remote files
func getRemoteMap(source string) (map[string]string, error) {
	// parse host and path
	parts := strings.SplitN(source, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid remote source %s", source)
	}
	host, path := parts[0], parts[1]
	// ensure no trailing slash in path for cd
	path = strings.TrimRight(path, "/")
	// build command
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
	// ensure no trailing slash
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

// padColumns formats four columns: status, filename, srcHash, dstHash
// syncDifferences syncs missing or mismatched files from source to destination
func syncDifferences(pair DirectoryPair) {
	// refresh maps
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
			// remote source
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
	return fmt.Sprintf("%-15s %-45s %-33s %-33s\n", status, file, src, dst)
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

// Bubbletea model
type Model struct {
	Pairs []DirectoryPair
	Index int
}
func NewModel(cfg Config) Model { return Model{Pairs: cfg.DirectoryPairs, Index: 0} }
func (m Model) Init() bubbletea.Cmd { return nil }
func (m Model) Update(msg bubbletea.Msg) (bubbletea.Model, bubbletea.Cmd) {
	// before handling keys, refresh differences by regenerating nothing (r will trigger re-render)

	switch msg := msg.(type) {
	case bubbletea.KeyMsg:
		switch msg.String() {
		case "q":
			return m, bubbletea.Quit
		case "tab":
			if m.Index < len(m.Pairs)-1 {
				m.Index++
			} else {
				m.Index = 0
			}
		case "r":
			// refresh: no state change, re-render
			return m, nil
		case "s":
			// sync missing/mismatch files
			syncDifferences(m.Pairs[m.Index])
			return m, nil
			// refresh: no state change needed, just re-render
			return m, nil
		}
	}
	return m, nil
}
func (m Model) View() string {
	if len(m.Pairs)==0 { return "No directory pairs\n" }
	p := m.Pairs[m.Index]
	head := fmt.Sprintf("Pair %d/%d %s -> %s\n", m.Index+1, len(m.Pairs), p.Source, p.Destination)
	body := highlightDifferences(p)
	return head + body + "\nPress tab to switch, r to refresh, s to sync, q to quit"
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
	// parse command-line flags
	noTUI := flag.Bool("no-tui", false, "output without TUI")
	flag.Parse()

	// load configuration
	cfg, err := loadConfig()
	if err != nil {
		fmt.Println("config:", err)
		os.Exit(1)
	}
	// non-TUI mode
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
	// TUI mode
	p := bubbletea.NewProgram(NewModel(cfg))
	if _, err := p.Run(); err!=nil { fmt.Println("run:",err) }
}
