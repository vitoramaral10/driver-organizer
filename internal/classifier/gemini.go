package classifier

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// Suggestion é a sugestão de classificação da IA para um arquivo.
type Suggestion struct {
	Filename        string  `json:"filename"`
	SuggestedFolder string  `json:"suggested_folder"`
	SuggestedName   string  `json:"suggested_name"`
	Reason          string  `json:"reason"`
	Confidence      float64 `json:"confidence"`
	NeedsContent    bool    `json:"needs_content"`
}

// Classifier é o cliente de classificação via Gemini API.
type Classifier struct {
	client    *genai.Client
	model     *genai.GenerativeModel
	modelName string
}

// NewClassifier cria um novo classificador usando a Gemini API.
func NewClassifier(ctx context.Context, apiKey, modelName string) (*Classifier, error) {
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("erro ao criar cliente Gemini: %w\n\nCertifique-se de:\n1. Ter uma API key do Google AI Studio\n2. Configurar via --gemini-api-key ou DORGANIZER_GEMINI_API_KEY\n3. Obtenha em: https://aistudio.google.com/apikey", err)
	}

	model := client.GenerativeModel(modelName)
	temp := float32(0.2)
	topP := float32(0.8)
	maxTokens := int32(4096)
	model.Temperature = &temp
	model.TopP = &topP
	model.MaxOutputTokens = &maxTokens

	// Instruções do sistema
	model.SystemInstruction = &genai.Content{
		Parts: []genai.Part{
			genai.Text(systemPrompt),
		},
	}

	// Forçar saída JSON
	model.ResponseMIMEType = "application/json"

	slog.Info("classificador Gemini inicializado", "model", modelName)
	return &Classifier{
		client:    client,
		model:     model,
		modelName: modelName,
	}, nil
}

// Close fecha o cliente.
func (c *Classifier) Close() {
	if c.client != nil {
		c.client.Close()
	}
}

// FileMetadata contém dados de um arquivo para classificação.
type FileMetadata struct {
	Name         string `json:"name"`
	MimeType     string `json:"mime_type"`
	Size         int64  `json:"size"`
	CreatedTime  string `json:"created_time"`
	ModifiedTime string `json:"modified_time"`
}

// ClassifyBatch classifica um lote de arquivos.
func (c *Classifier) ClassifyBatch(ctx context.Context, files []FileMetadata, existingFolders []string) ([]Suggestion, error) {
	prompt := buildClassificationPrompt(files, existingFolders)

	slog.Debug("enviando prompt de classificação", "files", len(files))

	resp, err := c.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("erro ao classificar arquivos: %w", err)
	}

	suggestions, err := parseResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("erro ao parsear resposta da IA: %w", err)
	}

	return suggestions, nil
}

// ClassifySingle classifica um único arquivo.
func (c *Classifier) ClassifySingle(ctx context.Context, file FileMetadata, existingFolders []string) (*Suggestion, error) {
	suggestions, err := c.ClassifyBatch(ctx, []FileMetadata{file}, existingFolders)
	if err != nil {
		return nil, err
	}

	if len(suggestions) == 0 {
		return &Suggestion{
			Filename:        file.Name,
			SuggestedFolder: "Outros",
			Reason:          "Não foi possível classificar",
			Confidence:      0,
		}, nil
	}

	return &suggestions[0], nil
}

// ClassifyWithContent classifica usando conteúdo adicional do arquivo.
func (c *Classifier) ClassifyWithContent(ctx context.Context, file FileMetadata, content string, existingFolders []string) (*Suggestion, error) {
	prompt := buildContentPrompt(file, content, existingFolders)

	resp, err := c.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("erro ao classificar com conteúdo: %w", err)
	}

	suggestions, err := parseResponse(resp)
	if err != nil {
		return nil, err
	}

	if len(suggestions) == 0 {
		return &Suggestion{
			Filename:        file.Name,
			SuggestedFolder: "Outros",
			Reason:          "Não foi possível classificar",
			Confidence:      0,
		}, nil
	}

	return &suggestions[0], nil
}

// ClassifyWithDescription classifica usando uma descrição fornecida pelo usuário.
func (c *Classifier) ClassifyWithDescription(ctx context.Context, file FileMetadata, userDescription string, existingFolders []string) (*Suggestion, error) {
	prompt := buildDescriptionPrompt(file, userDescription, existingFolders)

	resp, err := c.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("erro ao classificar com descrição: %w", err)
	}

	suggestions, err := parseResponse(resp)
	if err != nil {
		return nil, err
	}

	if len(suggestions) == 0 {
		return &Suggestion{
			Filename:        file.Name,
			SuggestedFolder: "Outros",
			Reason:          "Não foi possível classificar",
			Confidence:      0,
		}, nil
	}

	return &suggestions[0], nil
}

func parseResponse(resp *genai.GenerateContentResponse) ([]Suggestion, error) {
	if resp == nil || len(resp.Candidates) == 0 {
		return nil, fmt.Errorf("resposta vazia da IA")
	}

	candidate := resp.Candidates[0]
	if candidate.Content == nil || len(candidate.Content.Parts) == 0 {
		return nil, fmt.Errorf("conteúdo vazio na resposta")
	}

	text, ok := candidate.Content.Parts[0].(genai.Text)
	if !ok {
		return nil, fmt.Errorf("resposta não é texto")
	}

	jsonStr := strings.TrimSpace(string(text))

	var suggestions []Suggestion
	if err := json.Unmarshal([]byte(jsonStr), &suggestions); err != nil {
		// Tentar parsear como objeto único
		var single Suggestion
		if err2 := json.Unmarshal([]byte(jsonStr), &single); err2 != nil {
			return nil, fmt.Errorf("erro ao parsear JSON: %w\nResposta: %s", err, jsonStr)
		}
		suggestions = []Suggestion{single}
	}

	return suggestions, nil
}

const systemPrompt = `Você é um assistente de organização de arquivos. Sua tarefa é analisar arquivos e sugerir a melhor pasta e nome para organizá-los.

Regras:
1. Analise o nome, tipo e metadados do arquivo
2. Sugira uma pasta existente quando fizer sentido, ou sugira criar uma nova
3. Use nomes de pasta em português, claros e concisos
4. Categorias comuns: Documentos, Fotos, Vídeos, Música, Projetos, Trabalho, Estudos, Financeiro, Pessoal, Configurações, Backups
5. Seja específico quando possível (ex: "Trabalho/Relatórios" ao invés de apenas "Trabalho")
6. Sugira um nome melhor quando o arquivo tiver nome genérico (ex: "documento.pdf", "IMG_1234.jpg", "Untitled.docx")
7. Mantenha o nome original se já for descritivo
8. Se não tiver certeza, marque needs_content como true e confidence baixo
9. Sempre retorne um array JSON válido

Responda SEMPRE em formato JSON array com objetos contendo:
- filename: nome original do arquivo
- suggested_folder: pasta sugerida (pode ser aninhada com /)
- suggested_name: nome sugerido (igual ao original se já for bom, ou melhor se for genérico)
- reason: razão breve da sugestão
- confidence: 0.0 a 1.0
- needs_content: true se precisar ver o conteúdo para melhor classificação`

func buildClassificationPrompt(files []FileMetadata, existingFolders []string) string {
	var sb strings.Builder

	sb.WriteString("Classifique os seguintes arquivos e sugira a melhor pasta para cada um.\n\n")

	if len(existingFolders) > 0 {
		sb.WriteString("Pastas já existentes (prefira usar estas quando fizer sentido):\n")
		for _, f := range existingFolders {
			sb.WriteString(fmt.Sprintf("- %s\n", f))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Arquivos para classificar:\n")
	for i, f := range files {
		sb.WriteString(fmt.Sprintf("%d. Nome: %s | Tipo: %s | Tamanho: %d bytes | Criado: %s\n",
			i+1, f.Name, f.MimeType, f.Size, f.CreatedTime))
	}

	sb.WriteString("\nRetorne um array JSON com a classificação de cada arquivo.")
	return sb.String()
}

func buildContentPrompt(file FileMetadata, content string, existingFolders []string) string {
	var sb strings.Builder

	sb.WriteString("Classifique o seguinte arquivo com base no nome, metadados E conteúdo.\n\n")

	if len(existingFolders) > 0 {
		sb.WriteString("Pastas já existentes:\n")
		for _, f := range existingFolders {
			sb.WriteString(fmt.Sprintf("- %s\n", f))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("Arquivo: %s\nTipo: %s\nTamanho: %d bytes\n\n", file.Name, file.MimeType, file.Size))

	// Limitar conteúdo a ~2000 chars para não estourar tokens
	if len(content) > 2000 {
		content = content[:2000] + "... [truncado]"
	}
	sb.WriteString(fmt.Sprintf("Conteúdo (primeiros caracteres):\n---\n%s\n---\n", content))
	sb.WriteString("\nRetorne um array JSON com a classificação.")

	return sb.String()
}

func buildDescriptionPrompt(file FileMetadata, userDescription string, existingFolders []string) string {
	var sb strings.Builder

	sb.WriteString("Classifique o seguinte arquivo com base no nome, metadados E descrição do usuário.\n\n")

	if len(existingFolders) > 0 {
		sb.WriteString("Pastas já existentes:\n")
		for _, f := range existingFolders {
			sb.WriteString(fmt.Sprintf("- %s\n", f))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("Arquivo: %s\nTipo: %s\nTamanho: %d bytes\n\n", file.Name, file.MimeType, file.Size))
	sb.WriteString(fmt.Sprintf("Descrição do usuário:\n---\n%s\n---\n\n", userDescription))
	sb.WriteString("Com base nesta descrição, sugira a melhor pasta e nome para o arquivo.\n")
	sb.WriteString("\nRetorne um array JSON com a classificação.")

	return sb.String()
}
