package cli

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vitoramaral10/driver-organizer/internal/config"
)

var (
	cfgFile string
	cfg     *config.Config
)

func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "driver-organizer",
		Short: "Organiza arquivos do Google Drive usando IA",
		Long: `Driver Organizer é uma ferramenta CLI que se conecta ao Google Drive
e organiza seus arquivos automaticamente usando Inteligência Artificial (Gemini).

Fluxo:
  1. Move todos os arquivos para uma pasta "backup"
  2. Analisa cada arquivo usando IA para sugerir a melhor pasta
  3. Pede confirmação antes de cada movimentação
  4. Reorganiza os arquivos em pastas organizadas`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return initConfig()
		},
	}

	// Flags globais
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "arquivo de configuração (padrão: ~/.config/driver-organizer/config.yaml)")
	rootCmd.PersistentFlags().String("log-level", "info", "nível de log (debug, info, warn, error)")
	rootCmd.PersistentFlags().Bool("dry-run", false, "simula operações sem mover arquivos")

	viper.BindPFlag("log_level", rootCmd.PersistentFlags().Lookup("log-level"))
	viper.BindPFlag("dry_run", rootCmd.PersistentFlags().Lookup("dry-run"))

	// Subcomandos
	rootCmd.AddCommand(newOrganizeCmd())
	rootCmd.AddCommand(newAuthCmd())

	return rootCmd
}

func Execute() {
	rootCmd := NewRootCmd()
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func initConfig() error {
	var err error
	cfg, err = config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("erro na configuração: %w", err)
	}

	// Setup logging
	level := slog.LevelInfo
	switch cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(handler))

	return nil
}
