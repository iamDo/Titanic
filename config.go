package main

// Config holds directory pairs
// mapstructure tags for viper
// Config is loaded from config.yaml
type Config struct {
	DirectoryPairs []DirectoryPair `mapstructure:"directory_pairs"`
}
