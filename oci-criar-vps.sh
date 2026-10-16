#!/usr/bin/env bash
# Tenta criar uma VM VM.Standard.A1.Flex (Always Free) na Oracle Cloud
# em loop, até a região ter capacidade.
#
# Pré-requisito: OCI CLI instalada e configurada (oci setup config),
# ou rodar dentro do Cloud Shell da Oracle (já vem autenticado).

set -u

# ================== CONFIGURAÇÃO ==================
OCPUS=2                      # free tier: até 4 OCPUs no total
MEMORY_GB=16                 # free tier: até 24 GB no total
BOOT_VOLUME_GB=50            # o free tier dá até 200 GB no total
DISPLAY_NAME="minha-vps"
SSH_PUBLIC_KEY_FILE="$HOME/.ssh/id_ed25519.pub"   # sua chave PÚBLICA
INTERVALO=60                 # segundos entre as tentativas

# Opcional: se você já sabe os OCIDs, preencha aqui. Se deixar vazio, o script descobre sozinho.
COMPARTMENT_ID=""
SUBNET_ID=""
IMAGE_ID=""
AVAILABILITY_DOMAIN=""
# ===================================================

SHAPE="VM.Standard.A1.Flex"

# A OCI CLI fica em ~/bin; garante que ela está no PATH mesmo dentro do tmux
export PATH="$HOME/bin:$PATH"

if ! command -v oci >/dev/null 2>&1; then
  echo "OCI CLI não encontrada. Instale em https://docs.oracle.com/iaas/Content/API/SDKDocs/cliinstall.htm" >&2
  exit 1
fi

if [ ! -f "$SSH_PUBLIC_KEY_FILE" ]; then
  echo "Chave pública não encontrada em $SSH_PUBLIC_KEY_FILE" >&2
  echo "Gere uma com: ssh-keygen -t ed25519" >&2
  exit 1
fi

# Compartimento: por padrão usa o root (tenancy)
if [ -z "$COMPARTMENT_ID" ]; then
  COMPARTMENT_ID=$(grep -m1 '^tenancy' ~/.oci/config 2>/dev/null | cut -d= -f2 | tr -d ' ')
  COMPARTMENT_ID="${COMPARTMENT_ID:-${OCI_CLI_TENANCY:-}}"
fi
if [ -z "$COMPARTMENT_ID" ]; then
  echo "Não consegui descobrir o compartimento. Preencha COMPARTMENT_ID no topo do script." >&2
  exit 1
fi

# Availability Domain (São Paulo só tem um)
if [ -z "$AVAILABILITY_DOMAIN" ]; then
  AVAILABILITY_DOMAIN=$(oci iam availability-domain list \
    --compartment-id "$COMPARTMENT_ID" \
    --query 'data[0].name' --raw-output)
fi

# Subnet: pega a primeira subnet do compartimento (a que o console criou pra você)
if [ -z "$SUBNET_ID" ]; then
  SUBNET_ID=$(oci network subnet list \
    --compartment-id "$COMPARTMENT_ID" \
    --query 'data[0].id' --raw-output)
fi

# Imagem: Ubuntu 22.04 compatível com ARM (A1)
if [ -z "$IMAGE_ID" ]; then
  IMAGE_ID=$(oci compute image list \
    --compartment-id "$COMPARTMENT_ID" \
    --operating-system "Canonical Ubuntu" \
    --operating-system-version "22.04" \
    --shape "$SHAPE" \
    --sort-by TIMECREATED --sort-order DESC \
    --query 'data[0].id' --raw-output)
fi

for var in AVAILABILITY_DOMAIN SUBNET_ID IMAGE_ID; do
  if [ -z "${!var}" ] || [ "${!var}" = "null" ]; then
    echo "Não consegui descobrir $var. Preencha manualmente no topo do script." >&2
    exit 1
  fi
done

echo "Compartimento: $COMPARTMENT_ID"
echo "AD:            $AVAILABILITY_DOMAIN"
echo "Subnet:        $SUBNET_ID"
echo "Imagem:        $IMAGE_ID"
echo "Shape:         $SHAPE ($OCPUS OCPU / $MEMORY_GB GB)"
echo "Tentando a cada ${INTERVALO}s. Ctrl+C pra parar."
echo

tentativa=0
while true; do
  tentativa=$((tentativa + 1))
  echo "[$(date '+%d/%m %H:%M:%S')] Tentativa #$tentativa..."

  saida=$(oci compute instance launch \
    --availability-domain "$AVAILABILITY_DOMAIN" \
    --compartment-id "$COMPARTMENT_ID" \
    --shape "$SHAPE" \
    --shape-config "{\"ocpus\": $OCPUS, \"memoryInGBs\": $MEMORY_GB}" \
    --image-id "$IMAGE_ID" \
    --subnet-id "$SUBNET_ID" \
    --assign-public-ip true \
    --display-name "$DISPLAY_NAME" \
    --boot-volume-size-in-gbs "$BOOT_VOLUME_GB" \
    --ssh-authorized-keys-file "$SSH_PUBLIC_KEY_FILE" 2>&1)
  codigo=$?

  if [ $codigo -eq 0 ]; then
    echo
    echo "Instância criada! 🎉"
    echo "$saida"
    echo
    echo "Pegue o IP público no console (Compute > Instances) e conecte com:"
    echo "  ssh -i <sua-chave-privada> ubuntu@IP_PUBLICO"
    exit 0
  fi

  if echo "$saida" | grep -qi "capacity"; then
    echo "  Sem capacidade ainda. Aguardando ${INTERVALO}s..."
    sleep "$INTERVALO"
  elif echo "$saida" | grep -qiE "TooManyRequests|429"; then
    echo "  Limite de requisições. Aguardando $((INTERVALO * 3))s..."
    sleep $((INTERVALO * 3))
  elif echo "$saida" | grep -qiE "timed? ?out|timeout|connection|InternalError|ServiceUnavailable|\"status\": 50[0-9]"; then
    echo "  Erro temporário (rede ou Oracle instável). Tentando de novo em ${INTERVALO}s..."
    echo "$saida" | tail -n 3 | sed 's/^/    /'
    sleep "$INTERVALO"
  else
    echo "  Erro diferente de falta de capacidade, parando:"
    echo "$saida"
    exit 1
  fi
done
