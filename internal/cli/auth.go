package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/vitoramaral10/driver-organizer/internal/drive"
)

func newAuthCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "auth",
		Short: "Autentica com o Google Drive",
		Long:  "Realiza o fluxo de autenticação OAuth2 com o Google Drive e salva o token localmente.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			fmt.Println("🔐 Iniciando autenticação com Google Drive...")
			fmt.Printf("   Usando credentials: %s\n", cfg.CredentialsPath)
			fmt.Printf("   Token será salvo em: %s\n\n", cfg.TokenPath)

			_, err := drive.NewService(ctx, cfg.CredentialsPath, cfg.TokenPath)
			if err != nil {
				return fmt.Errorf("falha na autenticação: %w", err)
			}

			fmt.Println("\n✅ Autenticação realizada com sucesso!")
			fmt.Println("   Agora você pode usar: driver-organizer organize")
			return nil
		},
	}
}
