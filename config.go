package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"github.com/nknorg/nkn-sdk-go"
)

// Config represents the configuration data
type Config struct {
	Seed  string `json:"seed"`
	Title string `json:"title"`
	Owner string `json:"owner"`
}

// NewConfig reads the configuration file from a specified location and populates defaults
func NewConfig(configFile string) (*Config, error) {
	// Check if the file exists
	_, err := os.Stat(configFile)
	if err != nil {
		if os.IsNotExist(err) {

			//generate a wallet
			newWallet, _ := nkn.NewAccount(nil)
			seedStr := hex.EncodeToString(newWallet.Seed())
			// Create a default configuration
			defaultConfig := &Config{Seed: seedStr, Title: "Unnamed Stream"}
			data, err := json.MarshalIndent(defaultConfig, "", "  ")
			if err != nil {
				return nil, err
			}
			err = os.WriteFile(configFile, data, 0644) // Set appropriate permissions
			if err != nil {
				return nil, fmt.Errorf("error creating config file: %w", err)
			}
			return defaultConfig, nil
		}
		return nil, err
	}

	// Open the existing file and decode data
	f, err := os.Open(configFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var cfg Config
	decoder := json.NewDecoder(f)
	err = decoder.Decode(&cfg)
	if err != nil {
		return nil, err
	}

	// Set defaults for missing fields
	if cfg.Title == "" {
		cfg.Title = "Unnamed Stream"
	}

	return &cfg, nil
}
