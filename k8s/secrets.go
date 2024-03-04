package k8s

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strings"

	"github.com/plumber-cd/argocd-cmp-replicator/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/printers"
)

func (c *Client) GetLabeledSecrets(ctx context.Context, namespace string) (*corev1.SecretList, error) {
	secrets, err := c.CoreV1().Secrets("").List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", types.ReplicatorLabel, "true"),
	})
	if err != nil {
		return nil, err
	}

	slog.Debug("Listed labeled secrets", "count", len(secrets.Items))

	filteredSecrets := &corev1.SecretList{
		Items: []corev1.Secret{},
	}

	for _, secret := range secrets.Items {
		slog.Debug(
			"Checking secret",
			"name", secret.Name,
			"namespace", secret.Namespace,
			"thisNamespace", namespace,
		)

		match := matchSecretImplicitly(secret, namespace) ||
			matchSecretByWildcard(secret, namespace) ||
			matchSecretByList(secret, namespace)

		if !match {
			slog.Debug(
				"Skipped secret",
				"name", secret.Name,
				"namespace", secret.Namespace,
				"thisNamespace", namespace,
				"allowedNamespacesStr", secret.Annotations[types.ReplicatorAnnotationAllowedNamespaces],
			)
			continue
		}

		filteredSecrets.Items = append(filteredSecrets.Items, secret)
	}

	return filteredSecrets, nil
}

func (c *Client) WriteSecretListManifests(ctx context.Context, namespace string, secrets *corev1.SecretList, writer io.Writer) error {
	printer := printers.YAMLPrinter{}
	for _, secret := range secrets.Items {
		newName := secret.Name + "-replicated-from-" + secret.Namespace
		if secret.Annotations[types.ReplicatorAnnotationReplicatedName] != "" {
			newName = secret.Annotations[types.ReplicatorAnnotationReplicatedName]
		}
		newLabels := secret.Labels
		if newLabels != nil {
			delete(newLabels, types.ReplicatorLabel)
		}
		newAnnotations := secret.Annotations
		if newAnnotations == nil {
			newAnnotations = map[string]string{}
		}
		delete(newAnnotations, types.ReplicatorAnnotationAllowedNamespaces)
		delete(newAnnotations, types.ReplicatorAnnotationReplicatedName)
		newAnnotations[types.ReplicatorAnnotationFromNamespace] = secret.Namespace
		newSecret := corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Secret",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:        newName,
				Namespace:   namespace,
				Labels:      newLabels,
				Annotations: newAnnotations,
			},
			Data: secret.Data,
		}
		if err := printer.PrintObj(&newSecret, writer); err != nil {
			return err
		}
	}
	return nil
}

func matchSecretImplicitly(secret corev1.Secret, namespace string) bool {
	allowedNamespacesStr := secret.Annotations[types.ReplicatorAnnotationAllowedNamespaces]
	match := (allowedNamespacesStr == "" || allowedNamespacesStr == "-") && secret.Namespace == namespace
	if match {
		slog.Debug(
			"Matched secret implicitly",
			"name", secret.Name,
			"namespace", secret.Namespace,
			"thisNamespace", namespace,
			"allowedNamespacesStr", allowedNamespacesStr,
		)
	}
	return match
}

func matchSecretByWildcard(secret corev1.Secret, namespace string) bool {
	allowedNamespacesStr := secret.Annotations[types.ReplicatorAnnotationAllowedNamespaces]
	match := allowedNamespacesStr == "*"
	if match {
		slog.Debug(
			"Matched secret by wildcard",
			"name", secret.Name,
			"namespace", secret.Namespace,
			"thisNamespace", namespace,
			"allowedNamespacesStr", allowedNamespacesStr,
		)
	}
	return match
}

func matchSecretByList(secret corev1.Secret, namespace string) bool {
	allowedNamespacesStr := secret.Annotations[types.ReplicatorAnnotationAllowedNamespaces]
	allowedNamespaces := []string{}

	if strings.Contains(allowedNamespacesStr, ",") {
		slog.Debug(
			"Secret has multiple allowed namespaces",
			"name", secret.Name,
			"namespace", secret.Namespace,
			"thisNamespace", namespace,
			"allowedNamespacesStr", allowedNamespacesStr,
		)
		allowedNamespaces = strings.Split(strings.TrimSpace(allowedNamespacesStr), ",")
	} else {
		slog.Debug(
			"Secret has single allowed namespace",
			"name", secret.Name,
			"namespace", secret.Namespace,
			"thisNamespace", namespace,
			"allowedNamespacesStr", allowedNamespacesStr,
		)
		allowedNamespaces = append(allowedNamespaces, allowedNamespacesStr)
	}

	match := slices.Contains(allowedNamespaces, namespace)
	if match {
		slog.Debug(
			"Matched secret explicitly by allowed namespaces annotation",
			"name", secret.Name,
			"namespace", secret.Namespace,
			"thisNamespace", namespace,
			"allowedNamespacesStr", allowedNamespacesStr,
		)
	}
	return match
}
