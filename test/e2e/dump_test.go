package e2e

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type dumpCommand struct {
	output string
	args   []string
	stream bool
}

func collectE2EDump(ctx context.Context) error {
	if repoRoot == "" {
		return fmt.Errorf("repository root not resolved")
	}

	dumpDirectory := filepath.Join(repoRoot, "test", "e2e", "artifacts", "cluster-dump")
	manifestDirectory := filepath.Join(dumpDirectory, "manifests")
	logDirectory := filepath.Join(dumpDirectory, "logs")
	if err := os.MkdirAll(manifestDirectory, 0o755); err != nil {
		return fmt.Errorf("create E2E dump manifest directory: %w", err)
	}
	if err := os.MkdirAll(logDirectory, 0o755); err != nil {
		return fmt.Errorf("create E2E dump log directory: %w", err)
	}

	commands := []dumpCommand{
		{
			output: filepath.Join(dumpDirectory, "cluster-info.log"),
			args: []string{
				"cluster-info",
				"dump",
				"--all-namespaces",
				"--output-directory",
				filepath.Join(dumpDirectory, "cluster-info"),
				"-o",
				"yaml",
			},
		},
		{output: filepath.Join(manifestDirectory, "resources.txt"), args: []string{"get", "all", "-A", "-o", "wide"}},
		{output: filepath.Join(manifestDirectory, "events.yaml"), args: []string{"get", "events", "-A", "--sort-by=.metadata.creationTimestamp", "-o", "yaml"}},
		{output: filepath.Join(manifestDirectory, "endpoints.yaml"), args: []string{"get", "endpoints", "-A", "-o", "yaml"}},
		{output: filepath.Join(manifestDirectory, "endpoint-slices.yaml"), args: []string{"get", "endpointslices.discovery.k8s.io", "-A", "-o", "yaml"}},
		{output: filepath.Join(manifestDirectory, "ingresses.yaml"), args: []string{"get", "ingresses.networking.k8s.io", "-A", "-o", "yaml"}},
		{output: filepath.Join(manifestDirectory, "ingress-classes.yaml"), args: []string{"get", "ingressclasses.networking.k8s.io", "-o", "yaml"}},
		{output: filepath.Join(manifestDirectory, "configmaps.yaml"), args: []string{"get", "configmaps", "-A", "-o", "yaml"}},
		{output: filepath.Join(manifestDirectory, "pods-describe.txt"), args: []string{"describe", "pods", "-A"}},
		{output: filepath.Join(manifestDirectory, "deployments-describe.txt"), args: []string{"describe", "deployments", "-A"}},
		{output: filepath.Join(manifestDirectory, "ingresses-describe.txt"), args: []string{"describe", "ingresses.networking.k8s.io", "-A"}},
		{
			output: filepath.Join(logDirectory, "controller.log"),
			args:   []string{"logs", "-n", "cloudflare-tunnel-ingress-controller", "-l", "app.kubernetes.io/instance=cf-ic-e2e", "--all-containers=true", "--prefix=true", "--tail=-1"},
			stream: true,
		},
		{
			output: filepath.Join(logDirectory, "controller-previous.log"),
			args:   []string{"logs", "-n", "cloudflare-tunnel-ingress-controller", "-l", "app.kubernetes.io/instance=cf-ic-e2e", "--all-containers=true", "--prefix=true", "--previous", "--tail=-1"},
			stream: true,
		},
		{
			output: filepath.Join(logDirectory, "cloudflared.log"),
			args:   []string{"logs", "-n", "cloudflare-tunnel-ingress-controller", "-l", "strrl.dev/cloudflare-tunnel-ingress-controller=controlled-cloudflared-connector", "--all-containers=true", "--prefix=true", "--tail=-1"},
			stream: true,
		},
		{
			output: filepath.Join(logDirectory, "cloudflared-previous.log"),
			args:   []string{"logs", "-n", "cloudflare-tunnel-ingress-controller", "-l", "strrl.dev/cloudflare-tunnel-ingress-controller=controlled-cloudflared-connector", "--all-containers=true", "--prefix=true", "--previous", "--tail=-1"},
			stream: true,
		},
	}

	var dumpErrors []error
	for _, command := range commands {
		if err := runDumpCommand(ctx, command); err != nil {
			dumpErrors = append(dumpErrors, err)
		}
	}
	if err := writeRedactedSecrets(ctx, filepath.Join(manifestDirectory, "secrets.json")); err != nil {
		dumpErrors = append(dumpErrors, err)
	}

	return errors.Join(dumpErrors...)
}

func runDumpCommand(ctx context.Context, command dumpCommand) error {
	output, err := os.Create(command.output)
	if err != nil {
		return fmt.Errorf("create dump output %s: %w", command.output, err)
	}
	defer func() { _ = output.Close() }()

	var writer io.Writer = output
	if command.stream {
		writer = io.MultiWriter(output, GinkgoWriter)
	}

	cmd := exec.CommandContext(ctx, "kubectl", command.args...)
	cmd.Stdout = writer
	cmd.Stderr = writer
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kubectl %v: %w", command.args, err)
	}
	return nil
}

func writeRedactedSecrets(ctx context.Context, outputPath string) error {
	secrets, err := kubeClient.CoreV1().Secrets("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("list secrets for E2E dump: %w", err)
	}
	for i := range secrets.Items {
		secrets.Items[i].Data = nil
		secrets.Items[i].StringData = nil
	}

	data, err := json.MarshalIndent(secrets, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal redacted secrets: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(outputPath, data, 0o600); err != nil {
		return fmt.Errorf("write redacted secrets: %w", err)
	}
	return nil
}
