package utility

import (
	"fmt"
	"github.com/joho/godotenv"
	"os"
	"path/filepath"
)

func getConfig(projectDir string) (map[string]string, error) {
	configFile := filepath.Join(projectDir, ".config")
	envMap, err := godotenv.Read(configFile)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	return envMap, nil
}

func updateConfig(projectDir, key, value string) error {
	envMap, err := getConfig(projectDir)
	if err != nil {
		return fmt.Errorf("error reading config file: %w", err)
	}

	envMap[key] = value

	// write goenv back to .config file

	if err := saveConfig(projectDir, envMap); err != nil {
		return fmt.Errorf("error saving config file: %w", err)
	}

	return nil
}

func saveConfig(projectDir string, config map[string]string) error {
	configFile := filepath.Join(projectDir, ".config")

	f, err := os.Open(configFile)
	if err != nil {
		return fmt.Errorf("error creating config file: %w", err)
	}
	defer f.Close()

	for key, value := range config {
		_, err := f.WriteString(fmt.Sprintf("%s=%s\n", key, value))
		if err != nil {
			return fmt.Errorf("error writing to config file: %w", err)
		}
	}

	return nil
}
