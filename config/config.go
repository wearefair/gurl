package config

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"sync"

	yaml "gopkg.in/yaml.v2"

	"github.com/wearefair/gurl/log"
)

const (
	configDir  = ".gurl"
	configFile = ".gurl/config"
)

var (
	instance *Config
	logger   = log.Logger()
	once     sync.Once
)

// Not implemented yet
type Github struct {
	PublicKey string `json:"public_key"`
}

type Config struct {
	configured bool
	Local      struct {
		ImportPaths  []string `json:"import_paths"`
		ServicePaths []string `json:"service_paths"`
	} `json:"local"`
}

func homeDir() string {
	usr, err := user.Current()
	if err != nil {
		logger.Fatal(err.Error())
	}
	return usr.HomeDir
}

func Instance() *Config {
	once.Do(func() {
		instance = &Config{}
	})
	return instance
}

// Reads in config file from $HOME/.grpccurl and returns the instance of the config
func Read() *Config {
	config := Instance()
	configPath := filepath.Join(homeDir(), configFile)
	if _, err := os.Stat(configPath); err != nil {
		return nil
	}
	contents, err := ioutil.ReadFile(configPath)
	if err != nil {
		logger.Fatal(err.Error())
	}
	err = yaml.Unmarshal(contents, config)
	if err != nil {
		logger.Fatal(err.Error())
	}
	return config
}

// Saves config file in $HOME/.grpccurl or returns an error
func Save(config *Config) error {
	configDir := filepath.Join(homeDir(), configDir)
	contents, err := yaml.Marshal(config)
	if err != nil {
		logger.Error(err.Error())
		return err
	}
	err = os.MkdirAll(configDir, 0744)
	if err != nil {
		logger.Error(err.Error())
		return err
	}
	return ioutil.WriteFile(filepath.Join(homeDir(), configFile), contents, 0644)
}

// Prompt user for config inputs
func Prompt() {
	config := Instance()
	reader := bufio.NewReader(os.Stdin)
	config.Local.ImportPaths = parsePath(reader, "Import paths (comma delimited)")
	config.Local.ServicePaths = parsePath(reader, "Service paths (comma delimited)")
}

func parsePath(reader *bufio.Reader, description string) []string {
	fmt.Println(description + ": ")
	val, err := reader.ReadString('\n')
	if err != nil {
		logger.Error(err.Error())
		return nil
	}
	// Strip newline
	val = val[:len(val)-1]
	if val == "" {
		return nil
	}
	return strings.Split(val, ",")
}
