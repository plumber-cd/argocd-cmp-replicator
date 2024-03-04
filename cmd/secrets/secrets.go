package secrets

import (
	"errors"
	"log/slog"
	"os"

	"github.com/plumber-cd/argocd-cmp-replicator/k8s"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	Cmd.PersistentFlags().String("namespace", "", "Namespace to search for secrets - this is ignored if ARGOCD_APP_NAMESPACE is set")
}

type K8sClient struct {
	*k8s.Client
}

// versionCmd will print the version
var Cmd = &cobra.Command{
	Use:   "secrets",
	Short: "Find secrets matching given criteria",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		namespace := os.Getenv("ARGOCD_APP_NAMESPACE")
		namespaceFromArg := viper.GetString("namespace")
		if namespace == "" {
			namespace = namespaceFromArg
		} else if namespaceFromArg != "" {
			slog.Error("Namespace is set as ARGOCD_APP_NAMESPACE, not allowed to set namespace as an argument")
		}
		if namespace == "" {
			slog.Error("Namespace not set")
			return errors.New("Namespace not set")
		}

		_client, err := k8s.New()
		if err != nil {
			slog.Error("Failed to create k8s client", "err", err)
			return err
		}

		client := K8sClient{
			_client,
		}

		secrets, err := client.GetLabeledSecrets(ctx, "")
		if err != nil {
			slog.Error("Failed to get secrets", "err", err)
			return err
		}

		slog.Info("Filtered secrets", "count", len(secrets.Items))

		if err := client.WriteSecretListManifests(ctx, namespace, secrets, os.Stdout); err != nil {
			slog.Error("Failed to write secrets", "err", err)
			return err
		}

		return nil
	},
}
