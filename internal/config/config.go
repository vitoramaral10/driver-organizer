package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	CredentialsPath string  `mapstructure:"credentials_path"`
	TokenPath       string  `mapstructure:"token_path"`
	GeminiAPIKey    string  `mapstructure:"gemini_api_key"`
	GeminiModel     string  `mapstructure:"gemini_model"`
	BackupFolder    string  `mapstructure:"backup_folder"`
	BatchSize       int     `mapstructure:"batch_size"`
	RateLimit       int     `mapstructure:"rate_limit"`
	MaxCost         float64 `mapstructure:"max_cost"`
	LogLevel        string  `mapstructure:"log_level"`
	DryRun          bool    `mapstructure:"dry_run"`
}

func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		CredentialsPath: filepath.Join(home, ".config", "driver-organizer", "credentials.json"),
		TokenPath:       filepath.Join(home, ".config", "driver-organizer", "token.json"),
		GeminiAPIKey:    "",
		GeminiModel:     "gemini-2.0-flash",
		BackupFolder:    "backup",
		BatchSize:       20,
		RateLimit:       10,
		MaxCost:         5.0,
		LogLevel:        "info",
		DryRun:          false,
	}
}

// ConfigDir retorna o diretório de configuração.
func ConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "driver-organizer")
}

// GeminiKeyPath retorna o caminho do arquivo que armazena a API key do Gemini.
func GeminiKeyPath() string {
	return filepath.Join(ConfigDir(), "gemini_api_key")
}

// LoadGeminiAPIKey carrega a API key salva em disco.
func LoadGeminiAPIKey() (string, error) {
	data, err := os.ReadFile(GeminiKeyPath())
	if err != nil {
		return "", err
	}
	key := strings.TrimSpace(string(data))
	if key == "" {
		return "", fmt.Errorf("arquivo de API key vazio")
	}
	return key, nil
}

// SaveGeminiAPIKey salva a API key em disco.
func SaveGeminiAPIKey(key string) error {
	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("erro ao criar diretório de config: %w", err)
	}
	return os.WriteFile(GeminiKeyPath(), []byte(strings.TrimSpace(key)), 0600)
}

func Load(cfgFile string) (*Config, error) {
	cfg := DefaultConfig()

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("erro ao obter diretório home: %w", err)
		}
		viper.AddConfigPath(filepath.Join(home, ".config", "driver-organizer"))
		viper.AddConfigPath(".")
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	viper.SetEnvPrefix("Dorganizer")
	viper.AutomaticEnv()

	// Defaults
	viper.SetDefault("credentials_path", cfg.CredentialsPath)
	viper.SetDefault("token_path", cfg.TokenPath)
	viper.SetDefault("gemini_api_key", cfg.GeminiAPIKey)
	viper.SetDefault("gemini_model", cfg.GeminiModel)
	viper.SetDefault("backup_folder", cfg.BackupFolder)
	viper.SetDefault("batch_size", cfg.BatchSize)
	viper.SetDefault("rate_limit", cfg.RateLimit)
	viper.SetDefault("max_cost", cfg.MaxCost)
	viper.SetDefault("log_level", cfg.LogLevel)
	viper.SetDefault("dry_run", cfg.DryRun)

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("erro ao ler config: %w", err)
		}
	}

	if err := viper.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("erro ao decodificar config: %w", err)
	}

	return cfg, nil
}
