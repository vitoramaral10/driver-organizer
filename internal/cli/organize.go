package cli

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/vitoramaral10/driver-organizer/internal/classifier"
	"github.com/vitoramaral10/driver-organizer/internal/config"
	"github.com/vitoramaral10/driver-organizer/internal/drive"
)

func newOrganizeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "organize",
		Short: "Organiza os arquivos do Google Drive",
		Long: `Move todos os arquivos para "backup" e depois reorganiza
usando IA para sugerir a melhor pasta para cada arquivo.`,
		RunE: runOrganize,
	}

	cmd.Flags().String("gemini-api-key", "", "API key do Google AI Studio para Gemini")
	cmd.Flags().String("gemini-model", "gemini-2.0-flash", "modelo Gemini a usar")
	cmd.Flags().String("backup-folder", "backup", "pasta de backup")
	cmd.Flags().Int("batch-size", 20, "arquivos por lote de classificação")
	cmd.Flags().Float64("max-cost", 5.0, "custo máximo estimado em USD")
	cmd.Flags().Bool("resume", false, "continua organizacao a partir da pasta de backup")

	viper.BindPFlag("gemini_api_key", cmd.Flags().Lookup("gemini-api-key"))
	viper.BindPFlag("gemini_model", cmd.Flags().Lookup("gemini-model"))
	viper.BindPFlag("backup_folder", cmd.Flags().Lookup("backup-folder"))
	viper.BindPFlag("batch_size", cmd.Flags().Lookup("batch-size"))
	viper.BindPFlag("max_cost", cmd.Flags().Lookup("max-cost"))

	return cmd
}

func runOrganize(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\n\n⚠️  Interrupção recebida, encerrando de forma segura...")
		cancel()
	}()

	dryRun := cfg.DryRun
	if dryRun {
		fmt.Println("🔍 MODO DRY-RUN: nenhum arquivo será movido")
		fmt.Println()
	}

	resume, err := cmd.Flags().GetBool("resume")
	if err != nil {
		return err
	}

	// === SETUP: Verificar API key do Gemini ===
	if err := ensureGeminiAPIKey(); err != nil {
		return err
	}

	// === SETUP: Verificar autenticação Drive ===
	fmt.Println("📁 Conectando ao Google Drive...")
	srv, err := drive.NewService(ctx, cfg.CredentialsPath, cfg.TokenPath)
	if err != nil {
		return err
	}

	// === ETAPA 2: Listar arquivos na raiz ===
	fmt.Println("📋 Listando arquivos na raiz do Drive...")
	allFiles, err := drive.ListAllFiles(ctx, srv)
	if err != nil {
		return fmt.Errorf("erro ao listar arquivos: %w", err)
	}

	// Separar arquivos e pastas
	var files []*drive.FileInfo
	var folders []*drive.FileInfo
	for _, f := range allFiles {
		if f.IsFolder() {
			folders = append(folders, f)
		} else {
			files = append(files, f)
		}
	}

	fmt.Printf("   Encontrados: %d arquivos e %d pastas\n\n", len(files), len(folders))

	// === ETAPA 3: Criar pasta de backup e mover arquivos ===
	fmt.Printf("📦 Criando pasta de backup '%s'...\n", cfg.BackupFolder)

	backupFolder, err := drive.FindOrCreateNestedFolder(ctx, srv, cfg.BackupFolder, "root")
	if err != nil {
		return fmt.Errorf("erro ao criar pasta de backup: %w", err)
	}

	// Filtrar: não mover a própria pasta de backup
	backupRootName := strings.Split(cfg.BackupFolder, "/")[0]
	var filesToBackup []*drive.FileInfo
	
	// Incluir arquivos
	for _, f := range files {
		if f.Name == backupRootName && f.IsFolder() {
			continue
		}
		filesToBackup = append(filesToBackup, f)
	}
	
	// Incluir pastas (exceto a pasta de backup)
	for _, f := range folders {
		if f.Name == backupRootName {
			continue
		}
		filesToBackup = append(filesToBackup, f)
	}

	if resume {
		fmt.Println("↩️  Modo continuar: usando arquivos da pasta de backup. Itens na raiz não serão movidos.")
		backupFiles, err := drive.ListAllFilesRecursive(ctx, srv, backupFolder.ID)
		if err != nil {
			return fmt.Errorf("erro ao listar arquivos no backup: %w", err)
		}

		if len(backupFiles) == 0 {
			fmt.Println("✅ Nenhum arquivo para organizar!")
			return nil
		}

		fmt.Printf("   Encontrados %d arquivos no backup (incluindo subpastas).\n\n", len(backupFiles))
		filesToBackup = backupFiles
	} else {
		// Se não há nada para mover, verificar se há arquivos no backup para organizar
		if len(filesToBackup) == 0 {
			fmt.Println("   Nenhum item novo na raiz. Verificando pasta de backup recursivamente...\n")

			backupFiles, err := drive.ListAllFilesRecursive(ctx, srv, backupFolder.ID)
			if err != nil {
				return fmt.Errorf("erro ao listar arquivos no backup: %w", err)
			}

			if len(backupFiles) == 0 {
				fmt.Println("✅ Nenhum arquivo para organizar!")
				return nil
			}

			fmt.Printf("   Encontrados %d arquivos no backup (incluindo subpastas).\n\n", len(backupFiles))
			filesToBackup = backupFiles
		} else {
			fmt.Printf("📦 Movendo %d arquivos e pastas para backup...\n", len(filesToBackup))

			if !dryRun {
				bar := progressbar.NewOptions(len(filesToBackup),
					progressbar.OptionSetDescription("   Backup"),
					progressbar.OptionSetWidth(40),
					progressbar.OptionShowCount(),
					progressbar.OptionSetTheme(progressbar.Theme{
						Saucer:        "█",
						SaucerPadding: "░",
						BarStart:      "[",
						BarEnd:        "]",
					}),
				)

				for _, f := range filesToBackup {
					if ctx.Err() != nil {
						return fmt.Errorf("operação cancelada")
					}

					oldParent := "root"
					if len(f.Parents) > 0 {
						oldParent = f.Parents[0]
					}

					if err := drive.MoveFile(ctx, srv, f.ID, backupFolder.ID, oldParent); err != nil {
						slog.Error("falha ao mover para backup", "file", f.Name, "error", err)
					}

					bar.Add(1)
				}
				fmt.Println()
			} else {
				for _, f := range filesToBackup {
					fmt.Printf("   [DRY-RUN] Moveria: %s → %s\n", f.Name, cfg.BackupFolder)
				}
			}
		}
	}

	// === ETAPA 4: Inicializar classificador IA ===
	fmt.Println("\n🤖 Inicializando classificador IA...")
	cls, err := classifier.NewClassifier(ctx, cfg.GeminiAPIKey, cfg.GeminiModel)
	if err != nil {
		return err
	}
	defer cls.Close()

	// Coletar nomes de pastas existentes na raiz
	rootFolders, err := drive.ListFolders(ctx, srv, "root")
	if err != nil {
		slog.Warn("erro ao listar pastas existentes", "error", err)
	}
	var existingFolderNames []string
	for _, f := range rootFolders {
		existingFolderNames = append(existingFolderNames, f.Name)
	}
	
	if len(existingFolderNames) > 0 {
		fmt.Printf("   Pastas existentes que a IA conhece: %d\n", len(existingFolderNames))
		slog.Info("pastas existentes carregadas", "count", len(existingFolderNames), "folders", existingFolderNames)
	}

	// Cache de classificações
	cache := classifier.NewCache()

	// Filtrar apenas arquivos para organização (pastas ficam no backup)
	var filesToOrganize []*drive.FileInfo
	for _, f := range filesToBackup {
		if !f.IsFolder() {
			filesToOrganize = append(filesToOrganize, f)
		}
	}

	if len(filesToOrganize) == 0 {
		fmt.Println("\n✅ Todos os itens foram movidos para backup. Nenhum arquivo para organizar!")
		return nil
	}

	// === ETAPA 5: Classificar e organizar arquivos ===
	fmt.Printf("\n🗂️  Iniciando organização de %d arquivos...\n", len(filesToOrganize))
	fmt.Println("   Para cada arquivo, você pode:")
	fmt.Println("   (m) Mover para pasta sugerida (e renomear se sugerido)")
	fmt.Println("   (d) Descrever o arquivo para a IA reanalisar")
	fmt.Println("   (r) Renomear a pasta de destino")
	fmt.Println("   (n) Alterar o nome do arquivo")
	fmt.Println("   (c) Criar nova pasta personalizada")
	fmt.Println("   (p) Pular arquivo")
	fmt.Println("   (q) Sair")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)
	organized := 0
	skipped := 0

	fileLoop:
	for i, f := range filesToOrganize {
		if ctx.Err() != nil {
			fmt.Printf("\n⚠️  Operação cancelada. %d/%d arquivos organizados.\n", organized, len(filesToOrganize))
			return nil
		}

		fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Printf("📄 [%d/%d] %s\n", i+1, len(filesToBackup), f.Name)
		fmt.Printf("   Tipo: %s | Tamanho: %s | Criado: %s\n", f.MimeType, formatSize(f.Size), f.CreatedTime)

		// Verificar cache
		cacheKey := classifier.CacheKey(f.Name, f.MimeType)
		suggestion := cache.Get(cacheKey)

		if suggestion == nil {
			// Classificar com IA
			meta := classifier.FileMetadata{
				Name:         f.Name,
				MimeType:     f.MimeType,
				Size:         f.Size,
				CreatedTime:  f.CreatedTime,
				ModifiedTime: f.ModifiedTime,
			}

			var err error
			suggestion, err = cls.ClassifySingle(ctx, meta, existingFolderNames)
			if err != nil {
				slog.Error("erro na classificação", "file", f.Name, "error", err)
				fmt.Printf("   ❌ Erro ao classificar: %v\n", err)
				fmt.Printf("   Pulando arquivo...\n\n")
				skipped++
				continue
			}

			cache.Set(cacheKey, suggestion)
		}

		fmt.Printf("\n   🤖 Sugestão da IA:\n")
		fmt.Printf("      Pasta: %s\n", suggestion.SuggestedFolder)
		if suggestion.SuggestedName != "" && suggestion.SuggestedName != f.Name {
			fmt.Printf("      Nome: %s → %s\n", f.Name, suggestion.SuggestedName)
		}
		fmt.Printf("      Motivo: %s\n", suggestion.Reason)
		fmt.Printf("      Confiança: %.0f%%\n", suggestion.Confidence*100)

		if suggestion.NeedsContent {
			fmt.Printf("      ⚠️  IA sugere analisar conteúdo para melhor classificação\n")
		}

		var targetFolder string
		var targetName string

		for {
			fmt.Printf("\n   Ação? (m)over / (d)escrever / (r)enomear pasta / (n)omear arquivo / (c)riar nova / (p)ular / (q)uit: ")

			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(strings.ToLower(input))

			switch input {
			case "m", "":
				targetFolder = suggestion.SuggestedFolder
				if suggestion.SuggestedName != "" && suggestion.SuggestedName != f.Name {
					targetName = suggestion.SuggestedName
				} else {
					targetName = f.Name
				}
				break

			case "d":
				fmt.Printf("   Descreva o arquivo (ex: relatório mensal de vendas): ")
				description, _ := reader.ReadString('\n')
				description = strings.TrimSpace(description)
				
				if description == "" {
					fmt.Println("   Descrição vazia, mantendo sugestão atual.")
				} else {
					fmt.Println("   🤖 Reanalisando com sua descrição...")
					
					meta := classifier.FileMetadata{
						Name:         f.Name,
						MimeType:     f.MimeType,
						Size:         f.Size,
						CreatedTime:  f.CreatedTime,
						ModifiedTime: f.ModifiedTime,
					}
					
					newSuggestion, err := cls.ClassifyWithDescription(ctx, meta, description, existingFolderNames)
					if err != nil {
						slog.Error("erro na reclassificação", "file", f.Name, "error", err)
						fmt.Printf("   ❌ Erro ao reclassificar: %v\n", err)
						fmt.Println("   Mantendo sugestão original.")
					} else {
						suggestion = newSuggestion
						cache.Set(cacheKey, suggestion)
						
						fmt.Printf("\n   🤖 Nova sugestão:\n")
						fmt.Printf("      Pasta: %s\n", suggestion.SuggestedFolder)
						if suggestion.SuggestedName != "" && suggestion.SuggestedName != f.Name {
							fmt.Printf("      Nome: %s → %s\n", f.Name, suggestion.SuggestedName)
						}
						fmt.Printf("      Motivo: %s\n", suggestion.Reason)
						fmt.Printf("      Confiança: %.0f%%\n\n", suggestion.Confidence*100)
					}
				}
				// Repergunta a ação sem mover automaticamente
				continue

			case "r":
			fmt.Printf("   Novo nome da pasta [%s]: ", suggestion.SuggestedFolder)
			newFolder, _ := reader.ReadString('\n')
			newFolder = strings.TrimSpace(newFolder)
			if newFolder == "" {
				targetFolder = suggestion.SuggestedFolder
			} else {
				targetFolder = newFolder
			}
			if suggestion.SuggestedName != "" && suggestion.SuggestedName != f.Name {
				targetName = suggestion.SuggestedName
			} else {
				targetName = f.Name
			}
			break

		case "n":
			targetFolder = suggestion.SuggestedFolder
			defaultName := f.Name
			if suggestion.SuggestedName != "" && suggestion.SuggestedName != f.Name {
				defaultName = suggestion.SuggestedName
			}
			fmt.Printf("   Novo nome do arquivo [%s]: ", defaultName)
			newName, _ := reader.ReadString('\n')
			newName = strings.TrimSpace(newName)
			if newName == "" {
				targetName = defaultName
			} else {
				targetName = newName
			}

		case "c":
			fmt.Printf("   Nome da nova pasta: ")
			customName, _ := reader.ReadString('\n')
			customName = strings.TrimSpace(customName)
			if customName == "" {
				fmt.Println("   Nome vazio, pulando...")
				skipped++
				continue
			}
			targetFolder = customName
			if suggestion.SuggestedName != "" && suggestion.SuggestedName != f.Name {
				targetName = suggestion.SuggestedName
			} else {
				targetName = f.Name
			}

		case "p":
			fmt.Println("   ⏭️  Pulado")
			skipped++
			continue fileLoop

		case "q":
			fmt.Printf("\n✅ Organização encerrada. %d organizados, %d pulados.\n", organized, skipped)
			return nil

		default:
			fmt.Println("   Opção inválida, pulando...")
			skipped++
			continue fileLoop
		}
		break
		}

		// Executar a ação de mover/renomear
		destFolder, err := drive.FindOrCreateNestedFolder(ctx, srv, targetFolder, "root")
		if err != nil {
			slog.Error("erro ao criar pasta destino", "folder", targetFolder, "error", err)
			fmt.Printf("   ❌ Erro ao criar pasta: %v\n", err)
			skipped++
			continue
		}

		// Obter o parent atual do arquivo
		oldParent := backupFolder.ID
		if len(f.Parents) > 0 {
			oldParent = f.Parents[0]
		}

		// Se mudou o nome, fazer move + rename
		if targetName != f.Name {
			if err := drive.MoveAndRenameFile(ctx, srv, f.ID, targetName, destFolder.ID, oldParent); err != nil {
				slog.Error("erro ao mover e renomear arquivo", "file", f.Name, "error", err)
				fmt.Printf("   ❌ Erro ao mover: %v\n", err)
				skipped++
				continue
			}
			fmt.Printf("   ✅ Renomeado para: %s\n", targetName)
			fmt.Printf("   ✅ Movido para: %s\n", targetFolder)
		} else {
			// Apenas mover
			if err := drive.MoveFile(ctx, srv, f.ID, destFolder.ID, oldParent); err != nil {
				slog.Error("erro ao mover arquivo", "file", f.Name, "error", err)
				fmt.Printf("   ❌ Erro ao mover: %v\n", err)
				skipped++
				continue
			}
			fmt.Printf("   ✅ Movido para: %s\n", targetFolder)
		}

		// Adicionar pasta à lista de existentes se for nova
		isNew := true
		for _, name := range existingFolderNames {
			if name == targetFolder {
				isNew = false
				break
			}
		}
		if isNew {
			existingFolderNames = append(existingFolderNames, targetFolder)
			slog.Debug("pasta adicionada ao histórico", "folder", targetFolder)
		}

		organized++
		fmt.Println()
	}

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("\n🎉 Organização concluída!\n")
	fmt.Printf("   ✅ Organizados: %d\n", organized)
	fmt.Printf("   ⏭️  Pulados: %d\n", skipped)
	fmt.Printf("   📁 Total: %d\n", len(filesToOrganize))

	return nil
}

func formatSize(bytes int64) string {
	if bytes == 0 {
		return "N/A"
	}

	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// ensureGeminiAPIKey verifica se a API key do Gemini está disponível.
// Se não estiver salva em disco nem configurada, pede ao usuário e salva.
func ensureGeminiAPIKey() error {
	// 1. Já configurada via flag/env/config yaml?
	if cfg.GeminiAPIKey != "" {
		return nil
	}

	// 2. Tentar carregar do arquivo salvo
	savedKey, err := config.LoadGeminiAPIKey()
	if err == nil && savedKey != "" {
		cfg.GeminiAPIKey = savedKey
		fmt.Println("🔑 API key do Gemini carregada.")
		return nil
	}

	// 3. Pedir ao usuário
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("🔑 Configuração da API Key do Gemini")
	fmt.Println()
	fmt.Println("  Você precisa de uma API key do Google AI Studio.")
	fmt.Println("  Obtenha gratuitamente em: https://aistudio.google.com/apikey")
	fmt.Println()
	fmt.Print("  Cole sua API key aqui: ")

	reader := bufio.NewReader(os.Stdin)
	key, _ := reader.ReadString('\n')
	key = strings.TrimSpace(key)

	if key == "" {
		return fmt.Errorf("API key não pode ser vazia")
	}

	// 4. Salvar em disco
	if err := config.SaveGeminiAPIKey(key); err != nil {
		slog.Warn("não foi possível salvar API key em disco", "error", err)
	} else {
		fmt.Printf("  ✅ API key salva em: %s\n", config.GeminiKeyPath())
	}

	cfg.GeminiAPIKey = key
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	return nil
}
