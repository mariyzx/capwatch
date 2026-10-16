# capwatch

Keeps trying to launch an Oracle Cloud **Always Free** ARM instance
(`VM.Standard.A1.Flex`) until the region has capacity.

Popular regions often answer with:

```
Out of capacity for shape VM.Standard.A1.Flex in availability domain AD-1
```

Capacity frees up at random moments, so the only reliable way to get an
instance is to keep asking. capwatch does that for you, at a pace that stays
well below the OCI API rate limits.

> **Status:** the current version is a pair of Bash scripts. A Go CLI is in
> progress and will replace them.

## Requirements

- Linux or WSL 2 (tested on Ubuntu)
- An Oracle Cloud account (Free Tier is enough)
- A VCN with a public subnet in the target compartment
  (Console → Networking → Virtual cloud networks → **Start VCN Wizard** →
  *Create VCN with Internet Connectivity*)
- `sudo` access, to install `curl`, `python3`, `python3-venv`, `tmux` and
  `openssh-client`

The setup script installs the [OCI CLI](https://docs.oracle.com/iaas/Content/API/SDKDocs/cliinstall.htm)
and creates an SSH key if you don't have one.

## Quick start

```bash
git clone <repo-url> capwatch
cd capwatch
./scripts/setup-oci-wsl.sh
```

The setup walks you through `oci setup config`. Have these at hand:

| Prompt | Where to find it |
|---|---|
| User OCID | Profile menu → *My profile* → OCID |
| Tenancy OCID | Profile menu → *Tenancy* → OCID |
| Region | e.g. `sa-saopaulo-1` |

Then upload the printed **public** API key under
*My profile → API keys → Add API key*.

Once the connection test passes, the retry loop starts inside a `tmux`
session:

```bash
tmux attach -t oci-vps          # watch it (detach with Ctrl+B, D)
tail -f ~/oci-vps/log.txt       # or follow the log
tmux kill-session -t oci-vps    # stop it
```

When the instance is created, connect with:

```bash
ssh -i ~/.ssh/id_ed25519 ubuntu@<public-ip>
```

## Configuration

Edit the variables at the top of `scripts/oci-criar-vps.sh`:

| Variable | Default | Notes |
|---|---|---|
| `OCPUS` | `2` | Always Free allows up to 4 OCPUs in total |
| `MEMORY_GB` | `16` | Always Free allows up to 24 GB in total |
| `BOOT_VOLUME_GB` | `50` | Always Free allows up to 200 GB in total |
| `INTERVALO` | `60` | Seconds between attempts |
| `COMPARTMENT_ID`, `SUBNET_ID`, `IMAGE_ID`, `AVAILABILITY_DOMAIN` | empty | Discovered automatically when empty |

Smaller shapes tend to find capacity sooner.

## How it works

1. Discovers the availability domain, the first subnet and the latest
   Ubuntu 22.04 ARM image in the compartment.
2. Calls `oci compute instance launch` once per interval.
3. Classifies the response:
   - **out of capacity** → waits and retries
   - **rate limited (429)** → waits three times longer
   - **timeouts and 5xx** → treated as transient, retries
   - **anything else** → stops and prints the error

## Stack

- Bash
- [OCI CLI](https://github.com/oracle/oci-cli)
- tmux

## Project layout

```
scripts/
├── setup-oci-wsl.sh   # installs dependencies, configures the OCI CLI, starts the loop
└── oci-criar-vps.sh   # the retry loop
```

## Security

Never commit `~/.oci/`, `.pem` files, SSH keys or `log.txt` (it contains
your tenancy and subnet OCIDs). The `.gitignore` covers these by default.
