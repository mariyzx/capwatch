#!/usr/bin/env bash
# Prepara o WSL (Ubuntu) pra rodar o oci-criar-vps.sh:
#   1. instala dependências e a OCI CLI
#   2. gera uma chave SSH (se você não tiver)
#   3. te guia na configuração da OCI CLI
#   4. inicia o script de criação da VPS dentro de uma sessão tmux
#
# Uso: deixe este arquivo na mesma pasta do oci-criar-vps.sh e rode:
#   chmod +x setup-oci-wsl.sh oci-criar-vps.sh
#   ./setup-oci-wsl.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORK_DIR="$HOME/oci-vps"
TMUX_SESSION="oci-vps"

if [ ! -f "$SCRIPT_DIR/oci-criar-vps.sh" ]; then
  echo "Não achei o oci-criar-vps.sh na mesma pasta deste script ($SCRIPT_DIR)." >&2
  exit 1
fi

echo "==> 1/4 Instalando dependências (vai pedir sua senha do sudo)..."
sudo apt-get update -y
sudo apt-get install -y curl python3 tmux openssh-client

echo
echo "==> 2/4 Instalando a OCI CLI..."
if ! command -v oci >/dev/null 2>&1 && [ ! -x "$HOME/bin/oci" ]; then
  bash -c "$(curl -fsSL https://raw.githubusercontent.com/oracle/oci-cli/master/scripts/install/install.sh)" \
    -- --accept-all-defaults
fi
export PATH="$HOME/bin:$PATH"
if ! command -v oci >/dev/null 2>&1; then
  echo "A OCI CLI não ficou no PATH. Feche e abra o terminal do WSL e rode este script de novo." >&2
  exit 1
fi
echo "OCI CLI: $(oci --version)"

echo
echo "==> 3/4 Chave SSH..."
if [ ! -f "$HOME/.ssh/id_ed25519.pub" ]; then
  mkdir -p "$HOME/.ssh" && chmod 700 "$HOME/.ssh"
  ssh-keygen -t ed25519 -f "$HOME/.ssh/id_ed25519" -N "" -C "oracle-vps"
  echo "Chave SSH criada em ~/.ssh/id_ed25519 (a pública termina em .pub)."
else
  echo "Você já tem uma chave SSH em ~/.ssh/id_ed25519.pub, vou usar ela."
fi

echo
echo "==> 4/4 Configurando a OCI CLI..."
if [ ! -f "$HOME/.oci/config" ]; then
  cat <<'EOF'

Agora o 'oci setup config' vai te perguntar umas coisas. Tenha em mãos:

  - User OCID:     Console > ícone do perfil (canto superior direito) > My profile > copiar o OCID
  - Tenancy OCID:  Console > ícone do perfil > Tenancy > copiar o OCID
  - Região:        sa-saopaulo-1
  - Generate a new API signing RSA key pair? -> responda Y (aceite os padrões)

EOF
  read -r -p "Pressione ENTER pra começar..." _
  oci setup config

  PUB_KEY="$HOME/.oci/oci_api_key_public.pem"
  echo
  echo "Agora você precisa cadastrar a chave pública da API no console:"
  echo "  Console > My profile > API keys > Add API key > Paste a public key"
  echo "  e cole o conteúdo abaixo:"
  echo "------------------------------------------------------------"
  cat "$PUB_KEY"
  echo "------------------------------------------------------------"
  read -r -p "Quando terminar de adicionar a chave no console, pressione ENTER..." _
else
  echo "Já existe ~/.oci/config, vou reaproveitar."
fi

oci setup repair-file-permissions --file "$HOME/.oci/config" >/dev/null 2>&1 || true

echo
echo "Testando conexão com a Oracle..."
TENANCY_ID=$(grep -m1 '^tenancy' "$HOME/.oci/config" | cut -d= -f2 | tr -d ' ')
if oci iam availability-domain list --compartment-id "$TENANCY_ID" >/dev/null 2>&1; then
  echo "Conexão OK!"
else
  echo "Não consegui autenticar na Oracle. Confira se a chave pública da API foi adicionada" >&2
  echo "no console (My profile > API keys) e rode este script de novo." >&2
  exit 1
fi

mkdir -p "$WORK_DIR"
cp "$SCRIPT_DIR/oci-criar-vps.sh" "$WORK_DIR/oci-criar-vps.sh"
chmod +x "$WORK_DIR/oci-criar-vps.sh"

echo
read -r -p "Quer iniciar as tentativas de criar a VPS agora? [S/n] " resp
resp="${resp:-S}"
if [[ "$resp" =~ ^[Ss]$ ]]; then
  tmux kill-session -t "$TMUX_SESSION" 2>/dev/null || true
  tmux new-session -d -s "$TMUX_SESSION" "$WORK_DIR/oci-criar-vps.sh; echo; read -p 'Fim. ENTER pra fechar...'"
  echo
  echo "Rodando dentro do tmux (sessão '$TMUX_SESSION')."
  echo "  Ver o andamento:  tmux attach -t $TMUX_SESSION"
  echo "  Sair sem parar:   Ctrl+B e depois D"
  echo "  Parar de vez:     tmux kill-session -t $TMUX_SESSION"
else
  echo
  echo "Beleza! Pra rodar quando quiser: $WORK_DIR/oci-criar-vps.sh"
fi
