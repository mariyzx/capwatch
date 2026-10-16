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

Tested on Linux and WSL 2 (Ubuntu).

## Quick start

Once the [prerequisites](#prerequisites) are in place:

```bash
go install github.com/mariyzx/capwatch/cmd/capwatch@latest
export PATH="$PATH:$(go env GOPATH)/bin"
capwatch run
```

When it succeeds, [connect to the instance](#connecting-to-the-instance).

## Prerequisites

- Go 1.27+
- An Oracle Cloud account (Free Tier is enough)
- `tmux` (optional, to keep capwatch running in the background)

Do everything below in your **home region** (the region you picked at
sign-up, shown in the console's top bar). Always Free resources can only be
created there.

### 1. SSH key

The instance only accepts SSH keys. Skip this step if `~/.ssh/id_ed25519.pub`
already exists:

```bash
ssh-keygen -t ed25519
```

### 2. OCI API key

capwatch talks to OCI with an API key, read from `~/.oci/config`.

1. Create the directory:

   ```bash
   mkdir -p ~/.oci && chmod 700 ~/.oci
   ```

2. In the console, open *Profile menu → My profile → API keys → Add API key*.
3. Choose **Generate API key pair**, click **Download private key**, then **Add**.
4. Move the downloaded key into `~/.oci` (on WSL, Windows downloads are under
   `/mnt/c/Users/<you>/Downloads`):

   ```bash
   mv ~/Downloads/<downloaded-key>.pem ~/.oci/oci_api_key.pem
   ```

5. Copy the *Configuration file preview* into `~/.oci/config` and replace the
   `key_file` placeholder with the key path:

   ```ini
   [DEFAULT]
   user=ocid1.user.oc1..<...>
   fingerprint=<...>
   tenancy=ocid1.tenancy.oc1..<...>
   region=<your-home-region>
   key_file=~/.oci/oci_api_key.pem
   ```

6. Restrict the permissions:

   ```bash
   chmod 600 ~/.oci/config ~/.oci/oci_api_key.pem
   ```

See [Security](#security) for ways to limit what this key can do.

### 3. Network

*Networking → Virtual cloud networks → Start VCN Wizard → Create VCN with
Internet Connectivity*, in the **root compartment**. This creates a public
subnet, and its default security list already allows SSH (port 22).

capwatch uses the first subnet it finds in the compartment. If you have more
than one VCN, or a private subnet, pass the right one with `--subnet`.

## Installation

```bash
go install github.com/mariyzx/capwatch/cmd/capwatch@latest
```

The binary is installed to `$(go env GOPATH)/bin` (usually `~/go/bin`). If
your shell can't find `capwatch`, add that directory to your `PATH`:

```bash
echo 'export PATH="$PATH:$(go env GOPATH)/bin"' >> ~/.bashrc   # or ~/.zshrc
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

Progress goes to stderr. When the launch succeeds, capwatch prints the
instance OCID to stdout and exits with `0`.

Getting capacity may take hours or days. Run it inside `tmux` so it survives
closing the terminal:

```bash
tmux new-session -d -s capwatch 'umask 077; capwatch run 2>&1 | tee -a ~/capwatch.log'
tail -f ~/capwatch.log            # follow progress
tmux kill-session -t capwatch     # stop it
```

### Connecting to the instance

The instance takes a minute or two to boot. Get its public IP in the console
(*Compute → Instances → capwatch → Public IP address*) and connect as
`ubuntu`:

```bash
ssh -i ~/.ssh/id_ed25519 ubuntu@<public-ip>
```

## Configuration

Every setting can come from a flag, an environment variable or a config file,
in this order of precedence:

1. Flags: `--max-backoff 15m`
2. Environment variables: `CAPWATCH_` + the flag name in upper case, with `_`
   instead of `-`: `CAPWATCH_MAX_BACKOFF=15m`
3. Config file: `~/.config/capwatch/config.toml` (or `$XDG_CONFIG_HOME/capwatch/config.toml`),
   using the flag names as keys
4. Defaults

```toml
ocpus = 4
memory = 24
interval = "2m"
subnet = "ocid1.subnet.oc1.<region>.<...>"
```

```bash
chmod 600 ~/.config/capwatch/config.toml
```

capwatch refuses to start when the config file is writable by other users, or
when the file or a `CAPWATCH_*` variable contains an unknown setting.

These settings are not secrets. OCI credentials stay in `~/.oci/config`.

### Settings

| Flag | Default | Notes |
|---|---|---|
| `--ocpus` | `2` | 1–4 |
| `--memory` | `16` | GB, 1–24 |
| `--boot-volume` | `50` | GB, 50–200 |
| `--name` | `capwatch` | Instance display name |
| `--ssh-key` | `~/.ssh/id_ed25519.pub` | Public key authorized on the instance |
| `--compartment` | tenancy (root) | Compartment OCID |
| `--subnet` | first subnet found | Subnet OCID |
| `--interval` | `1m` | Wait between attempts, at least `30s` |
| `--max-backoff` | `10m` | Longest wait when rate limited |

The image is always the latest Ubuntu 22.04 for ARM. Smaller shapes tend to
find capacity sooner.

## Costs

The Always Free tier covers up to **4 OCPUs, 24 GB of memory and 200 GB of
block storage in total**, across all your instances. capwatch keeps each
request within these limits, but it does not check what you already have.

- **Free Tier accounts** can't go over the limits: OCI rejects the launch.
- **Pay As You Go accounts** are billed for anything above them. Check your
  existing A1 instances and volumes before running capwatch.

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
scripts/             # deprecated Bash version, kept for reference
```

## Development

```bash
go test -race ./...
go vet ./...
```

## Security

- capwatch never reads or stores credentials: authentication is handled by
  the OCI SDK from `~/.oci/config`.
- The API key gives access to your OCI account with your permissions, and the
  console generates it **without a passphrase**. Keep `~/.oci` at `700` and its
  files at `600`. On WSL, Windows programs running as your user can still read
  it.
- To limit the damage of a leaked key, create a dedicated OCI user for
  capwatch, in a group whose policy only allows managing instances and reading
  the network.
- capwatch only needs the key until the instance exists. Afterwards, delete it
  under *My profile → API keys* and remove the `.pem` file.
- `--ssh-key` only accepts a valid public key, so a private key is never sent
  to the instance metadata by mistake.
- The config file must not be writable by other users.
- Logs may contain OCIDs: create them with `umask 077`, as in the `tmux`
  example above.
- Never commit `~/.oci/`, `.pem` files, SSH keys or log files. The
  `.gitignore` covers these by default.
