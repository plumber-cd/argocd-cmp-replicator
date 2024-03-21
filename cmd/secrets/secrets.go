package secrets

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	argocdv1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/plumber-cd/argocd-cmp-replicator/k8s"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"sigs.k8s.io/yaml"
)

func init() {
	Cmd.PersistentFlags().String("namespace", "", "Namespace to search for secrets - this is ignored if ARGOCD_APP_NAMESPACE is set")
	Cmd.PersistentFlags().StringP("alternative-label-selector", "l", "", "This is a list of key=value pairs. If set, will override default label selector")
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

		alternativeLabelSelector := ""
		if v, ok := os.LookupEnv("ARGOCD_APP_PARAMETERS"); ok {
			if viper.GetString("alternative-label-selector") != "" {
				slog.Error("Both ARGOCD_APP_PARAMETERS and --alternative-label-selector were set")
				return fmt.Errorf("Both ARGOCD_APP_PARAMETERS and --alternative-label-selector were set")
			}
			slog.Debug("ARGOCD_APP_PARAMETERS", "value", v)
			params := argocdv1alpha1.ApplicationSourcePluginParameters{}
			if err := yaml.Unmarshal([]byte(v), &params); err != nil {
				return err
			}
			for _, param := range params {
				if param.Name == "alternative-label-selector" {
					if param.String_ == nil {
						slog.Error("alternative-label-selector is not a string")
						return fmt.Errorf("alternative-label-selector is not a string")
					}
					alternativeLabelSelector = *param.String_
					break
				}
			}
		} else {
			_alternativeLabelSelector, err := cmd.Flags().GetString("alternative-label-selector")
			if err != nil {
				return err
			}
			alternativeLabelSelector = _alternativeLabelSelector
		}

		_client, err := k8s.New()
		if err != nil {
			slog.Error("Failed to create k8s client", "err", err)
			return err
		}

		client := K8sClient{
			_client,
		}

		secrets, err := client.GetLabeledSecrets(ctx, namespace, alternativeLabelSelector)
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
