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

## Requirements

- Go 1.27+ (to build)
- An Oracle Cloud account (Free Tier is enough)
- An OCI API key configured in `~/.oci/config`
- A VCN with a public subnet in the target compartment
- An SSH key pair (default: `~/.ssh/id_ed25519.pub`)

### Setting up the OCI API key

1. In the console, open *Profile menu → My profile → API keys → Add API key*.
2. Choose **Generate API key pair**, download the private key and click **Add**.
3. Copy the *Configuration file preview* into `~/.oci/config` and point
   `key_file` to the downloaded key:

```ini
[DEFAULT]
user=ocid1.user.oc1..<...>
fingerprint=<...>
tenancy=ocid1.tenancy.oc1..<...>
region=sa-saopaulo-1
key_file=~/.oci/oci_api_key.pem
```

```bash
chmod 600 ~/.oci/config ~/.oci/oci_api_key.pem
```

### Creating the network

*Networking → Virtual cloud networks → Start VCN Wizard → Create VCN with
Internet Connectivity.* The default security list already allows SSH (port 22).

## Installation

```bash
go install github.com/mariyzx/capwatch/cmd/capwatch@latest
```

## Usage

```bash
capwatch run
```

```
Discovering availability domain, subnet and image...
Launching VM.Standard.A1.Flex (2 OCPUs, 16 GB) in Abcd:SA-SAOPAULO-1-AD-1, retrying every 1m0s
21:49:40 attempt 1: out of capacity, retrying in 1m5s
21:50:46 attempt 2: out of capacity, retrying in 1m6s
```

When the launch succeeds, the instance OCID is printed to stdout and the
command exits with `0`. Progress goes to stderr, so the output can be piped.

Getting capacity may take hours or days. Run it inside `tmux` so it survives
closing the terminal:

```bash
tmux new-session -d -s capwatch 'umask 077; capwatch run 2>&1 | tee -a ~/capwatch.log'
tail -f ~/capwatch.log
```

## Configuration

Every setting can come from a flag, an environment variable or the config
file, in this order of precedence:

1. Flags: `--max-backoff 15m`
2. Environment variables: `CAPWATCH_MAX_BACKOFF=15m`
3. Config file: `~/.config/capwatch/config.toml` (or `$XDG_CONFIG_HOME/capwatch/config.toml`)
4. Defaults

The config file uses the flag names as keys:

```toml
ocpus = 4
memory = 24
interval = "2m"
subnet = "ocid1.subnet.oc1.sa-saopaulo-1.<...>"
```

```bash
chmod 600 ~/.config/capwatch/config.toml
```

capwatch refuses to start when the config file is writable by other users, or
when the file or a `CAPWATCH_*` variable contains an unknown setting.

These settings are not secrets. OCI credentials stay in `~/.oci/config` and
are read only by the OCI SDK.

### Settings

| Flag | Default | Notes |
|---|---|---|
| `--ocpus` | `2` | 1–4 (Always Free limit) |
| `--memory` | `16` | GB, 1–24 (Always Free limit) |
| `--boot-volume` | `50` | GB, 50–200 |
| `--name` | `capwatch` | Instance display name |
| `--ssh-key` | `~/.ssh/id_ed25519.pub` | Public key authorized on the instance |
| `--compartment` | tenancy | Compartment OCID |
| `--subnet` | first subnet found | Subnet OCID |
| `--interval` | `1m` | Wait between attempts, at least `30s` |
| `--max-backoff` | `10m` | Longest wait when rate limited |

Smaller shapes tend to find capacity sooner.

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Instance launched |
| `1` | Unrecoverable error (authentication, quota, invalid settings...) |
| `130` | Interrupted with Ctrl+C |

## How it works

1. Discovers the availability domain, the first subnet and the latest
   Ubuntu 22.04 ARM image in the compartment.
2. Calls `LaunchInstance` once per interval, with a small random jitter.
3. Classifies each failure:
   - **out of capacity** and **transient** errors (timeouts, 5xx) → retries
     after the interval
   - **rate limited** (429) → doubles the wait, up to `--max-backoff`
   - **anything else** → stops and prints the error
4. Reuses the same retry token after transient failures, so a launch that
   timed out but succeeded on OCI's side is not duplicated.

## Stack

- [Go](https://go.dev)
- [OCI Go SDK](https://github.com/oracle/oci-go-sdk)
- [Cobra](https://github.com/spf13/cobra) + [Fang](https://github.com/charmbracelet/fang) for the CLI
- [Koanf](https://github.com/knadh/koanf) for configuration

## Project layout

```
cmd/capwatch/        # entrypoint
internal/
├── cli/             # cobra commands
│   └── run/         # capwatch run
├── config/          # flags, environment and config file loading
├── hunt/            # retry and backoff policy, independent of OCI
└── oci/             # OCI SDK adapter: discovery, launch, error translation
scripts/             # original Bash version
```

## Development

```bash
go test -race ./...
go vet ./...
```

## Security

- capwatch never reads or stores credentials: authentication is handled by
  the OCI SDK from `~/.oci/config`. Keep that directory at `700` and its
  files at `600`.
- `--ssh-key` only accepts a valid public key, so a private key is never
  sent to the instance metadata by mistake.
- The config file must not be writable by other users.
- Logs may contain OCIDs: create them with `umask 077`, as in the `tmux`
  example above.
- Never commit `~/.oci/`, `.pem` files, SSH keys or log files. The
  `.gitignore` covers these by default.
