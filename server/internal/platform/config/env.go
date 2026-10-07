// Package config loads local and process configuration without exposing secret
// values through errors or diagnostic output.
package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Values is an immutable view of parsed key/value configuration.
type Values struct {
	values map[string]string
}

// LoadFile parses a dotenv-style file. Blank lines and full-line comments are
// ignored. Duplicate and malformed keys are rejected without including their
// values in returned errors.
func LoadFile(path string) (Values, error) {
	file, err := os.Open(path)
	if err != nil {
		return Values{}, fmt.Errorf("open configuration file: %w", err)
	}
	defer file.Close()

	parsed := make(map[string]string)
	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		separator := strings.IndexByte(line, '=')
		if separator < 1 {
			return Values{}, fmt.Errorf("invalid configuration at line %d", lineNumber)
		}

		key := strings.TrimSpace(line[:separator])
		if !validKey(key) {
			return Values{}, fmt.Errorf("invalid configuration key at line %d", lineNumber)
		}
		if _, exists := parsed[key]; exists {
			return Values{}, fmt.Errorf("duplicate configuration key %s", key)
		}

		value := strings.TrimSpace(line[separator+1:])
		value = trimMatchingQuotes(value)
		parsed[key] = value
	}
	if err := scanner.Err(); err != nil {
		return Values{}, fmt.Errorf("read configuration file: %w", err)
	}

	return Values{values: parsed}, nil
}

// Lookup returns a value without adding it to error or log messages.
func (v Values) Lookup(key string) (string, bool) {
	value, ok := v.values[key]
	return value, ok
}

// Required returns a non-empty configured value.
func (v Values) Required(key string) (string, error) {
	value, ok := v.Lookup(key)
	if !ok || strings.TrimSpace(value) == "" || value == "CHANGE_ME" {
		return "", fmt.Errorf("required configuration key %s is missing", key)
	}
	return value, nil
}

// Int returns a required base-10 integer.
func (v Values) Int(key string) (int, error) {
	value, err := v.Required(key)
	if err != nil {
		return 0, err
	}
	parsed, parseErr := strconv.Atoi(value)
	if parseErr != nil {
		return 0, fmt.Errorf("configuration key %s must be an integer", key)
	}
	return parsed, nil
}

// Duration returns a required Go duration, such as 30s or 5m.
func (v Values) Duration(key string) (time.Duration, error) {
	value, err := v.Required(key)
	if err != nil {
		return 0, err
	}
	parsed, parseErr := time.ParseDuration(value)
	if parseErr != nil {
		return 0, fmt.Errorf("configuration key %s must be a duration", key)
	}
	return parsed, nil
}

// CSV returns a required comma-separated list with whitespace removed.
func (v Values) CSV(key string) ([]string, error) {
	value, err := v.Required(key)
	if err != nil {
		return nil, err
	}

	items := strings.Split(value, ",")
	result := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			result = append(result, item)
		}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("configuration key %s must contain at least one item", key)
	}
	return result, nil
}

func validKey(key string) bool {
	for index, character := range key {
		if character == '_' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' && index > 0 {
			continue
		}
		return false
	}
	return key != ""
}

func trimMatchingQuotes(value string) string {
	if len(value) < 2 {
		return value
	}
	if value[0] == value[len(value)-1] && (value[0] == '\'' || value[0] == '"') {
		return value[1 : len(value)-1]
	}
	return value
}
