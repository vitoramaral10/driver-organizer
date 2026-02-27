# Driver Organizer

🤖 Sistema CLI que organiza automaticamente seus arquivos do Google Drive usando Inteligência Artificial (Gemini).

## 📋 O que faz?

O Driver Organizer conecta ao seu Google Drive e ajuda a organizar arquivos de forma inteligente:

1. **Backup automático**: Move todos os arquivos para uma pasta de backup segura
2. **Classificação com IA**: Para cada arquivo, o Gemini analisa nome, tipo e metadados e sugere a melhor pasta
3. **Controle total**: Você confirma cada movimentação antes de acontecer
4. **Organização inteligente**: Cria pastas automaticamente quando necessário ou usa pastas existentes

## ✨ Características

- ✅ Classificação inteligente com Google Gemini
- ✅ Confirmação interativa antes de mover arquivos
- ✅ Modo dry-run para testar sem modificar nada
- ✅ Sugestões baseadas em pastas já existentes
- ✅ Backup automático antes de organizar
- ✅ Progress bar para operações longas
- ✅ Retry automático em caso de erros de rede
- ✅ Cache de classificações para evitar chamadas repetidas

## 🔧 Pré-requisitos

- **Go 1.21+** ([download](https://go.dev/dl/))
- **Conta Google** (para Google Drive)
- **API Key do Gemini** (gratuita)

## 📦 Instalação

### 1. Clone o repositório

```bash
git clone https://github.com/vitoramaral10/driver-organizer.git
cd driver-organizer
```

### 2. Instale as dependências

```bash
go mod download
```

### 3. Compile o projeto

```bash
go build -o driver-organizer cmd/driver-organizer/main.go
```

Ou no Windows:

```powershell
go build -o driver-organizer.exe cmd/driver-organizer/main.go
```

## 🔑 Configuração

### Passo 1: Obter API Key do Gemini

A API key do Gemini é **gratuita** e necessária para classificar os arquivos.

1. Acesse: [Google AI Studio](https://aistudio.google.com/apikey)
2. Faça login com sua conta Google
3. Clique em **"Create API Key"** ou **"Get API Key"**
4. Copie a API key gerada

⚠️ **Importante**: Guarde esta API key em segurança. Ela será solicitada na primeira execução.

### Passo 2: Criar Credenciais do Google Drive

Para acessar seus arquivos do Google Drive, você precisa criar credenciais OAuth2.

#### 2.1. Criar Projeto no Google Cloud Console

1. Acesse: [Google Cloud Console](https://console.cloud.google.com/)
2. Clique em **"Select a project"** → **"New Project"**
3. Nome do projeto: `Driver Organizer` (ou qualquer nome)
4. Clique em **"Create"**
5. Aguarde a criação e selecione o projeto criado

#### 2.2. Habilitar Google Drive API

1. No menu lateral, vá em **"APIs & Services"** → **"Library"**
2. Busque por **"Google Drive API"**
3. Clique na API e depois em **"Enable"**

#### 2.3. Criar Credenciais OAuth2

1. No menu lateral, vá em **"APIs & Services"** → **"Credentials"**
2. Clique em **"Create Credentials"** → **"OAuth client ID"**
3. Se aparecer aviso sobre OAuth consent screen:
   - Clique em **"Configure Consent Screen"**
   - Escolha **"External"** → **"Create"**
   - Preencha apenas:
     - **App name**: Driver Organizer
     - **User support email**: seu email
     - **Developer contact**: seu email
   - Clique em **"Save and Continue"** até **"Back to Dashboard"**
4. Volte para **"Credentials"** → **"Create Credentials"** → **"OAuth client ID"**
5. **Application type**: Escolha **"Web application"**
6. **Name**: Driver Organizer
7. Em **"Authorized redirect URIs"**, clique em **"ADD URI"** e adicione:
   ```
   http://localhost:8080
   ```
8. Clique em **"Create"**
9. Clique em **"Download JSON"** e salve o arquivo

#### 2.4. Salvar as Credenciais

Mova o arquivo JSON baixado para:

```bash
# Linux/Mac
~/.config/driver-organizer/credentials.json

# Windows
C:\Users\SeuUsuario\.config\driver-organizer\credentials.json
```

Ou crie o diretório se não existir:

```bash
# Linux/Mac
mkdir -p ~/.config/driver-organizer
mv ~/Downloads/client_secret_*.json ~/.config/driver-organizer/credentials.json
```

```powershell
# Windows PowerShell
New-Item -ItemType Directory -Force -Path "$env:USERPROFILE\.config\driver-organizer"
Move-Item "$env:USERPROFILE\Downloads\client_secret_*.json" "$env:USERPROFILE\.config\driver-organizer\credentials.json"
```

## 🚀 Como Usar

### Primeira Execução

Na primeira vez que você rodar, o sistema fará automaticamente:

1. **Pedirá a API Key do Gemini**: Cole a API key obtida anteriormente (será salva para próximas execuções)
2. **Abrirá o navegador**: Para você autorizar o acesso ao Google Drive
   - Uma aba será aberta automaticamente
   - Faça login com sua conta Google
   - Clique em **"Permitir"** para autorizar o acesso
   - Aguarde a mensagem de sucesso (pode fechar a aba)
3. **Token salvo**: O token será salvo automaticamente

```bash
./driver-organizer organize
```

✨ **Nas próximas execuções, tudo funcionará automaticamente sem pedir nada!**

### Comandos Disponíveis

#### `organize` - Organizar arquivos

Comando principal que organiza seus arquivos do Google Drive:

```bash
./driver-organizer organize
```

**Flags opcionais:**

```bash
# Usar uma API key específica (pula a solicitação)
./driver-organizer organize --gemini-api-key "sua-api-key-aqui"

# Usar modelo diferente do Gemini
./driver-organizer organize --gemini-model "gemini-pro"

# Mudar pasta de backup (padrão: "backup")
./driver-organizer organize --backup-folder "arquivos_antigos"

# Modo dry-run (simula sem mover arquivos)
./driver-organizer organize --dry-run

# Alterar tamanho do lote de classificação (padrão: 20)
./driver-organizer organize --batch-size 10

# Nível de log detalhado
./driver-organizer organize --log-level debug
```

#### `auth` - Autenticar com Google Drive

Força uma nova autenticação com o Google Drive (útil se o token expirou):

```bash
./driver-organizer auth
```

### Fluxo Interativo

Durante a organização, para cada arquivo você verá:

```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📄 [1/10] relatorio_vendas_2024.pdf
   Tipo: application/pdf | Tamanho: 2.3 MB | Criado: 2024-01-15

   🤖 Sugestão da IA:
      Pasta: Trabalho/Relatórios
      Motivo: Documento profissional relacionado a vendas
      Confiança: 95%

   Ação? (m)over / (r)enomear pasta / (c)riar nova / (p)ular / (q)uit:
```

**Opções:**
- **m** ou Enter: Move para a pasta sugerida
- **r**: Renomeia a pasta de destino
- **c**: Cria uma nova pasta com nome personalizado
- **p**: Pula este arquivo
- **q**: Sai do programa

## ⚙️ Configuração Avançada

### Arquivo de Configuração

Você pode criar um arquivo de configuração em `~/.config/driver-organizer/config.yaml`:

```yaml
# API Key do Gemini (opcional, será solicitada se não configurada)
gemini_api_key: "sua-api-key"

# Modelo Gemini a usar (padrão: gemini-2.0-flash)
gemini_model: "gemini-2.0-flash"

# Pasta de backup (padrão: backup)
backup_folder: "old/backup"

# Arquivos por lote na classificação (padrão: 20)
batch_size: 20

# Taxa limite de requisições por segundo (padrão: 10)
rate_limit: 10

# Custo máximo estimado em USD (padrão: 5.0)
max_cost: 5.0

# Nível de log: debug, info, warn, error (padrão: info)
log_level: "info"

# Modo dry-run (padrão: false)
dry_run: false
```

### Variáveis de Ambiente

Você também pode configurar via variáveis de ambiente (prefixo `DORGANIZER_`):

```bash
export DORGANIZER_GEMINI_API_KEY="sua-api-key"
export DORGANIZER_GEMINI_MODEL="gemini-2.0-flash"
export DORGANIZER_BACKUP_FOLDER="backup"
export DORGANIZER_LOG_LEVEL="debug"
export DORGANIZER_DRY_RUN="true"
```

### Prioridade de Configuração

A configuração é aplicada na seguinte ordem (maior prioridade primeiro):

1. **Flags de linha de comando** (`--gemini-api-key`, etc.)
2. **Variáveis de ambiente** (`DORGANIZER_*`)
3. **Arquivo de configuração** (`config.yaml`)
4. **Valores padrão**

## 🔒 Segurança

- **API Key do Gemini**: Salva em `~/.config/driver-organizer/gemini_api_key` (permissões 0600)
- **Token OAuth2 do Drive**: Salvo em `~/.config/driver-organizer/token.json` (permissões 0600)
- **Credenciais OAuth2**: Em `~/.config/driver-organizer/credentials.json` (permissões 0600)

⚠️ **Nunca compartilhe estes arquivos!** Adicione `.config/` ao seu `.gitignore` se for versionar.

## 💰 Custos

O Driver Organizer usa a API do Gemini, que tem **nível gratuito generoso**:

- **Gemini 2.0 Flash** (padrão):
  - Grátis: 1500 requisições/dia
  - ~1K tokens por classificação
  - Praticamente ilimitado para uso pessoal

Para 1000 arquivos: ~50 requisições (usando batch de 20) = **GRÁTIS**

## 🐛 Solução de Problemas

### Erro: "credentials.json não encontrado"

**Solução**: Siga o [Passo 2: Criar Credenciais do Google Drive](#passo-2-criar-credenciais-do-google-drive)

### Erro: "gemini_api_key não configurada"

**Solução**: Execute `./driver-organizer organize` e cole a API key quando solicitado, ou configure via flag/env/config.

### Erro: "Token has been expired or revoked"

**Solução**: Execute `./driver-organizer auth` para reautenticar.

### Erro: "403 Forbidden" no Drive

**Solução**: Verifique se a Google Drive API está habilitada no seu projeto GCP.

### Classificações ruins da IA

**Solução**: 
- Use `--batch-size 1` para classificações individuais (mais lento)
- Mude para `--gemini-model "gemini-pro"` (mais preciso, porém mais lento)
- Use a opção "r" ou "c" para corrigir manualmente

## 📝 Exemplos de Uso

### Organizar arquivos com dry-run primeiro

```bash
# Ver o que seria feito sem modificar nada
./driver-organizer organize --dry-run

# Se estiver OK, rodar de verdade
./driver-organizer organize
```

### Usar pasta de backup diferente

```bash
./driver-organizer organize --backup-folder "antigo/arquivos_desorganizados"
```

### Modo debug para troubleshooting

```bash
./driver-organizer organize --log-level debug
```

## 🤝 Contribuindo

Contribuições são bem-vindas! Sinta-se à vontade para abrir issues ou pull requests.

## 📄 Licença

Este projeto é de código aberto e está disponível sob sua licença de escolha.

## 👤 Autor

Vitor Amaral ([@vitoramaral10](https://github.com/vitoramaral10))

---

**⭐ Se este projeto foi útil, considere dar uma estrela no GitHub!**
