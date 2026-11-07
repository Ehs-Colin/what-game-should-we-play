package config

import (
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

const (
	defaultPort                  = 3030
	defaultDatabasePath          = ""
	defaultDatabaseFilename      = "wgswp.sqlite"
	defaultAppOrigin             = "http://localhost:5959"
	activeEncryptionVersionEnv   = "DATA_ENCRYPTION_ACTIVE_VERSION"
	encryptionKeyEnvPrefix       = "DATA_ENCRYPTION_KEY_"
	defaultSteamApiTimeout       = 5
	defaultSteamApiCacheInterval = 5
)

type Config struct {
	Port                  int
	DatabasePath          string
	AppOrigin             string
	SteamApiKey           string
	SteamApiTimeout       time.Duration
	SteamApiCacheInterval time.Duration
	//ActiveEncryptionKeyVersion string
	//EncryptionKeys             map[string][32]byte
}

func Load(workingDirectory string) (Config, error) {
	environment, err := loadEnvironment(filepath.Join(workingDirectory, ".env"))
	if err != nil {
		return Config{}, err
	}

	return Parse(environment, workingDirectory)
}

func loadEnvironment(dotenvPath string) (map[string]string, error) {
	environment := make(map[string]string)
	dotenvValues, err := godotenv.Read(dotenvPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read %s: %w", dotenvPath, err)
	}
	maps.Copy(environment, dotenvValues)
	for _, entry := range os.Environ() {
		name, value, found := strings.Cut(entry, "=")
		if found {
			environment[name] = value
		}
	}
	return environment, nil
}

func Parse(environment map[string]string, workingDirectory string) (Config, error) {
	port, err := parseNonNegativeInteger(valueOrDefault(environment, "PORT", strconv.Itoa(defaultPort)), "PORT")
	if err != nil {
		return Config{}, err
	}
	if port > 65_535 {
		return Config{}, errors.New("PORT must be no greater than 65535")
	}

	databasePath := environment["DATABASE_URL"]
	if databasePath == "" {
		databasePath = filepath.Join(workingDirectory, "data", defaultDatabaseFilename)
	}

	appOrigin, err := parseOrigin(valueOrDefault(environment, "APP_ORIGIN", defaultAppOrigin))
	if err != nil {
		return Config{}, err
	}
	// activeEncryptionKeyVersion, encryptionKeys, err := parseEncryptionKeys(environment)
	// if err != nil {
	// 	return Config{}, err
	// }
	steamApiKey, err := requireEnvironmentVariable(environment, "STEAM_API_KEY")
	if err != nil {
		return Config{}, err
	}
	steamApiTimeout, err := parseNonNegativeInteger(valueOrDefault(environment, "STEAM_API_TIMEOUT", strconv.Itoa(int(defaultSteamApiTimeout))), "STEAM_API_TIMEOUT")
	if err != nil {
		return Config{}, err
	}
	steamApiCacheInterval, err := parseNonNegativeInteger(valueOrDefault(environment, "STEAM_API_CACHE_INTERVAL", strconv.Itoa(int(defaultSteamApiCacheInterval))), "STEAM_API_CACHE_INTERVAL")
	if err != nil {
		return Config{}, err
	}

	return Config{
		Port:                  port,
		DatabasePath:          databasePath,
		AppOrigin:             appOrigin,
		SteamApiKey:           steamApiKey,
		SteamApiTimeout:       time.Duration(int64(steamApiTimeout) * int64(time.Second)),
		SteamApiCacheInterval: time.Duration(int64(steamApiCacheInterval) * int64(time.Second)),
		//ActiveEncryptionKeyVersion: activeEncryptionKeyVersion,
		//EncryptionKeys:             encryptionKeys,
	}, nil
}

func requireEnvironmentVariable(environment map[string]string, name string) (string, error) {
	value := environment[name]
	if value == "" {
		return "", fmt.Errorf("missing required environment variable: %s", name)
	}
	return value, nil
}

func valueOrDefault(environment map[string]string, name, fallback string) string {
	if value := environment[name]; value != "" {
		return value
	}
	return fallback
}

func parseOrigin(value string) (string, error) {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("APP_ORIGIN must be an absolute URL")
	}
	return parsed.Scheme + "://" + parsed.Host, nil
}

func parseNonNegativeInteger(value, name string) (int, error) {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("%s must be a non-negative integer", name)
	}
	return parsed, nil
}

func parseEncryptionKeys(environment map[string]string) (string, map[string][32]byte, error) {
	configuredVersion := environment[activeEncryptionVersionEnv]
	if configuredVersion == "" {
		return "", nil, fmt.Errorf("missing required environment variable: %s", activeEncryptionVersionEnv)
	}
	activeVersion, err := normalizeEncryptionVersion(configuredVersion)
	if err != nil {
		return "", nil, err
	}

	keys := make(map[string][32]byte)
	for name, value := range environment {
		if !strings.HasPrefix(name, encryptionKeyEnvPrefix) {
			continue
		}
		version, err := normalizeEncryptionVersion(strings.TrimPrefix(name, encryptionKeyEnvPrefix))
		if err != nil {
			return "", nil, err
		}
		if _, exists := keys[version]; exists {
			return "", nil, fmt.Errorf("duplicate encryption key version: %s", version)
		}
		key, err := parseEncryptionKey(value, name)
		if err != nil {
			return "", nil, err
		}
		keys[version] = key
	}
	if _, exists := keys[activeVersion]; !exists {
		return "", nil, fmt.Errorf("no encryption key configured for active version: %s", activeVersion)
	}
	return activeVersion, keys, nil
}

func parseEncryptionKey(value, name string) ([32]byte, error) {
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != 32 {
		return [32]byte{}, fmt.Errorf("%s must be exactly 64 hexadecimal characters", name)
	}
	return [32]byte(decoded), nil
}

func normalizeEncryptionVersion(version string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(version))
	if normalized == "" {
		return "", fmt.Errorf("invalid encryption key version: %s", version)
	}
	for index, character := range normalized {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' {
			continue
		}
		if character != '_' || index == 0 || index == len(normalized)-1 || normalized[index-1] == '_' {
			return "", fmt.Errorf("invalid encryption key version: %s", version)
		}
	}
	return normalized, nil
}
