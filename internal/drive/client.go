package drive

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

// NewService cria um novo serviço autenticado do Google Drive.
func NewService(ctx context.Context, credentialsPath, tokenPath string) (*drive.Service, error) {
	b, err := os.ReadFile(credentialsPath)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler credentials: %w\n\nBaixe o arquivo credentials.json do Google Cloud Console:\nhttps://console.cloud.google.com/apis/credentials", err)
	}

	config, err := google.ConfigFromJSON(b, drive.DriveScope)
	if err != nil {
		return nil, fmt.Errorf("erro ao parsear credentials: %w", err)
	}

	// Detectar tipo de credencial e ajustar RedirectURL
	var credData map[string]interface{}
	if err := json.Unmarshal(b, &credData); err == nil {
		// Verificar se tem "installed" (Desktop app) ou "web" (Web app)
		if installed, ok := credData["installed"].(map[string]interface{}); ok {
			// Desktop app - usar redirect padrão do Google
			if redirects, ok := installed["redirect_uris"].([]interface{}); ok && len(redirects) > 0 {
				config.RedirectURL = redirects[0].(string)
			}
		} else if web, ok := credData["web"].(map[string]interface{}); ok {
			// Web app - garantir que localhost:8080 está configurado
			config.RedirectURL = "http://localhost:8080"
			if redirects, ok := web["redirect_uris"].([]interface{}); ok && len(redirects) > 0 {
				// Procurar por localhost na lista
				for _, r := range redirects {
					if rStr, ok := r.(string); ok && (rStr == "http://localhost:8080" || rStr == "http://localhost:8080/") {
						config.RedirectURL = rStr
						break
					}
				}
			}
		}
	}

	client, err := getClient(ctx, config, tokenPath)
	if err != nil {
		return nil, fmt.Errorf("erro ao obter client OAuth2: %w", err)
	}

	srv, err := drive.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("erro ao criar serviço Drive: %w", err)
	}

	slog.Info("serviço Google Drive conectado com sucesso")
	return srv, nil
}

func getClient(ctx context.Context, config *oauth2.Config, tokenPath string) (*http.Client, error) {
	tok, err := tokenFromFile(tokenPath)
	if err != nil {
		tok, err = getTokenFromWeb(ctx, config, false)
		if err != nil {
			return nil, err
		}
		if err := saveToken(tokenPath, tok); err != nil {
			slog.Warn("não foi possível salvar token", "error", err)
		}
		return config.Client(ctx, tok), nil
	}

	// Sem refresh token, forcar novo consentimento para obter um.
	if tok.RefreshToken == "" {
		slog.Warn("token sem refresh_token, reautenticando com consentimento")
		tok, err = getTokenFromWeb(ctx, config, true)
		if err != nil {
			return nil, err
		}
		if err := saveToken(tokenPath, tok); err != nil {
			slog.Warn("não foi possível salvar token", "error", err)
		}
	}

	return config.Client(ctx, tok), nil
}

func getTokenFromWeb(ctx context.Context, config *oauth2.Config, forceConsent bool) (*oauth2.Token, error) {
	// Verificar se é Desktop app (redirect para urn:ietf:wg:oauth:2.0:oob) ou Web app (localhost)
	isDesktopApp := config.RedirectURL == "urn:ietf:wg:oauth:2.0:oob" || config.RedirectURL == "oob"
	
	if isDesktopApp {
		// Fluxo antigo para Desktop app - usuário copia código manualmente
		return getTokenManual(ctx, config, forceConsent)
	}
	
	// Fluxo moderno com servidor local para Web app
	return getTokenWithLocalServer(ctx, config, forceConsent)
}

// getTokenManual - fluxo manual para Desktop app
func getTokenManual(ctx context.Context, config *oauth2.Config, forceConsent bool) (*oauth2.Token, error) {
	authURL := buildAuthURL(config, forceConsent)
	
	fmt.Println("\n🔐 Autorização do Google Drive")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Println("  Abra este link no navegador:")
	fmt.Printf("  %s\n", authURL)
	fmt.Println()
	fmt.Print("  Cole o código de autorização aqui: ")
	
	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		return nil, fmt.Errorf("erro ao ler código: %w", err)
	}
	
	tok, err := config.Exchange(ctx, authCode)
	if err != nil {
		return nil, fmt.Errorf("erro ao trocar código por token: %w", err)
	}
	
	fmt.Println("  ✅ Token obtido com sucesso!")
	fmt.Println()
	
	return tok, nil
}

// getTokenWithLocalServer - fluxo automático com servidor local para Web app
func getTokenWithLocalServer(ctx context.Context, config *oauth2.Config, forceConsent bool) (*oauth2.Token, error) {
	// Canal para receber o código de autorização
	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)
	
	// Servidor HTTP temporário para capturar o callback
	server := &http.Server{Addr: ":8080"}
	
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "Código de autorização não encontrado", http.StatusBadRequest)
			errChan <- fmt.Errorf("código não encontrado na URL")
			return
		}
		
		// Página de sucesso
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `
			<!DOCTYPE html>
			<html>
			<head>
				<meta charset="utf-8">
				<title>Autenticação Concluída</title>
				<style>
					body { 
						font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
						display: flex;
						justify-content: center;
						align-items: center;
						height: 100vh;
						margin: 0;
						background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
					}
					.container {
						background: white;
						padding: 40px;
						border-radius: 10px;
						box-shadow: 0 10px 40px rgba(0,0,0,0.2);
						text-align: center;
						max-width: 400px;
					}
					h1 { color: #4CAF50; margin-top: 0; }
					p { color: #666; line-height: 1.6; }
					.emoji { font-size: 64px; margin: 20px 0; }
				</style>
			</head>
			<body>
				<div class="container">
					<div class="emoji">✅</div>
					<h1>Autenticação Concluída!</h1>
					<p>Você pode fechar esta janela e voltar ao terminal.</p>
					<p style="font-size: 14px; color: #999; margin-top: 20px;">
						Driver Organizer está pronto para usar.
					</p>
				</div>
			</body>
			</html>
		`)
		
		codeChan <- code
	})
	
	// Inicia o servidor em goroutine
	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			slog.Error("erro no servidor HTTP", "error", err)
		}
	}()
	
	// Aguarda um momento para o servidor iniciar
	time.Sleep(100 * time.Millisecond)
	
	authURL := buildAuthURL(config, forceConsent)
	
	fmt.Println("\n🔐 Autorização do Google Drive")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Println("  Abrindo navegador para autenticação...")
	fmt.Println()
	fmt.Println("  Se o navegador não abrir automaticamente, acesse:")
	fmt.Printf("  %s\n", authURL)
	fmt.Println()
	fmt.Println("  Aguardando autorização...")
	
	// Tenta abrir o navegador automaticamente
	openBrowser(authURL)
	
	// Aguarda o código ou timeout
	var authCode string
	select {
	case authCode = <-codeChan:
		// Código recebido com sucesso
	case err := <-errChan:
		server.Shutdown(context.Background())
		return nil, err
	case <-time.After(5 * time.Minute):
		server.Shutdown(context.Background())
		return nil, fmt.Errorf("timeout aguardando autorização (5 minutos)")
	case <-ctx.Done():
		server.Shutdown(context.Background())
		return nil, fmt.Errorf("operação cancelada")
	}
	
	// Desliga o servidor
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server.Shutdown(shutdownCtx)
	
	fmt.Println()
	fmt.Println("  ✅ Autorização recebida! Obtendo token...")
	
	// Troca o código pelo token
	tok, err := config.Exchange(ctx, authCode)
	if err != nil {
		return nil, fmt.Errorf("erro ao trocar código por token: %w", err)
	}
	
	fmt.Println("  ✅ Token obtido com sucesso!")
	fmt.Println()
	
	return tok, nil
}

func buildAuthURL(config *oauth2.Config, forceConsent bool) string {
	options := []oauth2.AuthCodeOption{oauth2.AccessTypeOffline}
	if forceConsent {
		options = append(options, oauth2.SetAuthURLParam("prompt", "consent"))
	}
	return config.AuthCodeURL("state-token", options...)
}

// openBrowser tenta abrir a URL no navegador padrão do sistema.
func openBrowser(url string) {
	var cmd *exec.Cmd
	
	switch runtime.GOOS {
	case "windows":
		// `start` treats the first quoted argument as the window title, so pass an empty title.
		cmd = exec.Command("cmd", "/c", "start", "", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default: // linux, freebsd, openbsd, netbsd
		cmd = exec.Command("xdg-open", url)
	}
	
	if err := cmd.Start(); err != nil {
		slog.Debug("não foi possível abrir navegador automaticamente", "error", err)
	}
}

func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

func saveToken(path string, token *oauth2.Token) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("erro ao criar diretório para token: %w", err)
	}

	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("erro ao criar arquivo de token: %w", err)
	}
	defer f.Close()

	return json.NewEncoder(f).Encode(token)
}
