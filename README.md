# capwatch

Fica tentando criar uma VM **Always Free** (`VM.Standard.A1.Flex`, ARM) na Oracle Cloud
até a região ter capacidade, resolvendo o famoso erro *"Out of capacity"*.

Feito para rodar no **WSL (Ubuntu)**.

## Scripts

- `setup-oci-wsl.sh`: instala a OCI CLI, prepara a chave SSH, configura a CLI e inicia o loop em uma sessão tmux.
- `oci-criar-vps.sh`: o loop que tenta criar a instância a cada minuto.

## Uso

```bash
./setup-oci-wsl.sh
```

> ⚠️ Nunca faça commit de `~/.oci/`, chaves `.pem`/SSH ou do `log.txt` (contém OCIDs da sua conta).
