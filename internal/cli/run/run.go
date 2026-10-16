package run

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"

	"github.com/mariyzx/capwatch/internal/config"
	"github.com/mariyzx/capwatch/internal/hunt"
	"github.com/mariyzx/capwatch/internal/oci"
)

const (
	shape       = "VM.Standard.A1.Flex"
	minInterval = 30 * time.Second
)

type Options struct {
	OCPUs         float32       `koanf:"ocpus"`
	MemoryGB      float32       `koanf:"memory"`
	BootVolumeGB  int64         `koanf:"boot-volume"`
	Name          string        `koanf:"name"`
	SSHKeyPath    string        `koanf:"ssh-key"`
	CompartmentID string        `koanf:"compartment"`
	SubnetID      string        `koanf:"subnet"`
	Interval      time.Duration `koanf:"interval"`
	MaxBackoff    time.Duration `koanf:"max-backoff"`
}

func NewCmdRun() *cobra.Command {
	var opts Options
	home, _ := os.UserHomeDir()

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Keep trying to launch the instance until it succeeds",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.Path()
			if err != nil {
				return err
			}
			if err := config.Load(path, cmd.Flags(), &opts); err != nil {
				return err
			}
			if err := opts.validate(); err != nil {
				return err
			}
			return runRun(cmd.Context(), opts, cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}

	flags := cmd.Flags()
	flags.Float32Var(&opts.OCPUs, "ocpus", 2, "number of OCPUs (Always Free: up to 4)")
	flags.Float32Var(&opts.MemoryGB, "memory", 16, "memory in GB (Always Free: up to 24)")
	flags.Int64Var(&opts.BootVolumeGB, "boot-volume", 50, "boot volume size in GB (Always Free: up to 200)")
	flags.StringVar(&opts.Name, "name", "capwatch", "instance display name")
	flags.StringVar(&opts.SSHKeyPath, "ssh-key", filepath.Join(home, ".ssh", "id_ed25519.pub"), "public SSH key authorized on the instance")
	flags.StringVar(&opts.CompartmentID, "compartment", "", "compartment OCID (default: tenancy)")
	flags.StringVar(&opts.SubnetID, "subnet", "", "subnet OCID (default: first subnet in the compartment)")
	flags.DurationVar(&opts.Interval, "interval", time.Minute, "wait between attempts")
	flags.DurationVar(&opts.MaxBackoff, "max-backoff", 10*time.Minute, "longest wait when rate limited")

	return cmd
}

func (o Options) validate() error {
	switch {
	case o.OCPUs < 1 || o.OCPUs > 4:
		return errors.New("invalid --ocpus: must be between 1 and 4")
	case o.MemoryGB < 1 || o.MemoryGB > 24:
		return errors.New("invalid --memory: must be between 1 and 24")
	case o.BootVolumeGB < 50 || o.BootVolumeGB > 200:
		return errors.New("invalid --boot-volume: must be between 50 and 200")
	case o.Interval < minInterval:
		return fmt.Errorf("invalid --interval: must be at least %s", minInterval)
	case o.MaxBackoff < o.Interval:
		return errors.New("invalid --max-backoff: must not be shorter than --interval")
	}
	return nil
}

func runRun(ctx context.Context, opts Options, out, errOut io.Writer) error {
	sshKey, err := readPublicKey(opts.SSHKeyPath)
	if err != nil {
		return err
	}

	client, err := oci.NewClient()
	if err != nil {
		return err
	}

	fmt.Fprintln(errOut, "Discovering availability domain, subnet and image...")
	compartmentID := cmp.Or(opts.CompartmentID, client.Tenancy())
	placement, err := client.Discover(ctx, compartmentID, shape)
	if err != nil {
		return err
	}
	placement.SubnetID = cmp.Or(opts.SubnetID, placement.SubnetID)

	fmt.Fprintf(errOut, "Launching %s (%g OCPUs, %g GB) in %s, retrying every %s\n",
		shape, opts.OCPUs, opts.MemoryGB, placement.AvailabilityDomain, opts.Interval)

	hunter := hunt.Hunter{
		Launcher: client.Launcher(oci.Spec{
			Placement:     placement,
			CompartmentID: compartmentID,
			Shape:         shape,
			OCPUs:         opts.OCPUs,
			MemoryGB:      opts.MemoryGB,
			BootVolumeGB:  opts.BootVolumeGB,
			DisplayName:   opts.Name,
			SSHPublicKey:  sshKey,
		}),
		Interval:   opts.Interval,
		MaxBackoff: opts.MaxBackoff,
		OnAttempt: func(a hunt.Attempt) {
			if a.Err != nil && a.Wait > 0 {
				fmt.Fprintf(errOut, "%s %s\n", time.Now().Format(time.TimeOnly), retryMessage(a))
			}
		},
	}

	instance, err := hunter.Run(ctx)
	if err != nil {
		return err
	}

	fmt.Fprintln(errOut, "Instance launched. It may take a minute to boot and get a public IP.")
	fmt.Fprintln(out, instance.ID)
	return nil
}

func readPublicKey(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading ssh public key: %w", err)
	}
	if _, _, _, _, err := ssh.ParseAuthorizedKey(content); err != nil {
		return "", fmt.Errorf("invalid --ssh-key: %s is not a public key (did you pass the private key?)", path)
	}
	return strings.TrimSpace(string(content)), nil
}

func retryMessage(a hunt.Attempt) string {
	reason := a.Err.Error()
	for _, known := range []error{hunt.ErrOutOfCapacity, hunt.ErrRateLimited, hunt.ErrTransient} {
		if errors.Is(a.Err, known) {
			reason = known.Error()
			break
		}
	}
	return fmt.Sprintf("attempt %d: %s, retrying in %s", a.Number, reason, a.Wait.Round(time.Second))
}
