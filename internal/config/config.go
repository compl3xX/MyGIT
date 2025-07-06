package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Config holds the configuration data, organized by sections.
type Config struct {
	path   string
	values map[string]map[string]string // section -> key -> value
}

// NewConfig creates a new Config instance.
func NewConfig(path string) *Config {
	return &Config{
		path:   path,
		values: make(map[string]map[string]string),
	}
}

// Load reads the configuration file and parses it.
func (c *Config) Load() error {
	file, err := os.Open(c.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	currentSection := ""
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			sectionName := strings.Trim(line, "[]")
			parts := strings.Fields(sectionName)
			if len(parts) > 1 {
				currentSection = fmt.Sprintf("%s.%s", parts[0], strings.Trim(parts[1], "\""))
			} else {
				currentSection = parts[0]
			}
			if _, ok := c.values[currentSection]; !ok {
				c.values[currentSection] = make(map[string]string)
			}
			continue
		}

		if currentSection != "" {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				c.values[currentSection][key] = value
			}
		}
	}
	return scanner.Err()
}

// Save writes the configuration to the file.
func (c *Config) Save() error {
	file, err := os.Create(c.path)
	if err != nil {
		return err
	}
	defer file.Close()

	for section, kv := range c.values {
		parts := strings.SplitN(section, ".", 2)
		if len(parts) == 2 {
			fmt.Fprintf(file, "[%s \"%s\"]\n", parts[0], parts[1])
		} else {
			fmt.Fprintf(file, "[%s]\n", section)
		}

		for key, value := range kv {
			fmt.Fprintf(file, "\t%s = %s\n", key, value)
		}
		fmt.Fprintln(file, "")
	}
	return nil
}

// Get returns a configuration value.
func (c *Config) Get(key string) (string, bool) {
	parts := strings.Split(key, ".")
	if len(parts) < 2 {
		return "", false
	}
	section := strings.Join(parts[:len(parts)-1], ".")
	actualKey := parts[len(parts)-1]

	if sectionValues, ok := c.values[section]; ok {
		val, ok := sectionValues[actualKey]
		return val, ok
	}
	return "", false
}

// Set sets a configuration value.
func (c *Config) Set(key, value string) {
	parts := strings.Split(key, ".")
	if len(parts) < 2 {
		return
	}
	section := strings.Join(parts[:len(parts)-1], ".")
	actualKey := parts[len(parts)-1]

	if _, ok := c.values[section]; !ok {
		c.values[section] = make(map[string]string)
	}
	c.values[section][actualKey] = value
}

// GetAll returns all configuration values in a flat map.
func (c *Config) GetAll() map[string]string {
	flat := make(map[string]string)
	for section, kv := range c.values {
		for key, value := range kv {
			flat[fmt.Sprintf("%s.%s", section, key)] = value
		}
	}
	return flat
}
