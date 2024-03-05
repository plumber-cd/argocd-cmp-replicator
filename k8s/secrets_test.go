package k8s

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"testing"

	"github.com/plumber-cd/argocd-cmp-replicator/types"
	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	testClient "k8s.io/client-go/kubernetes/fake"
)

//go:embed testdata/secrets.yaml
var secretsYAML string

func TestMatchSecretImplicitly(t *testing.T) {
	t.Run("match-implicitly", func(t *testing.T) {
		secret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "labeled-secret",
				Namespace: "my-test-namespace",
			},
		}

		match := matchSecretImplicitly(secret, "my-test-namespace")
		require.True(t, match)
	})
	t.Run("match-implicitly-with-annotation", func(t *testing.T) {
		secret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "labeled-secret",
				Namespace: "my-test-namespace",
				Annotations: map[string]string{
					types.ReplicatorAnnotationAllowedNamespaces: "-",
				},
			},
		}

		match := matchSecretImplicitly(secret, "my-test-namespace")
		require.True(t, match)
	})
	t.Run("do-not-match", func(t *testing.T) {
		secret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "labeled-secret",
				Namespace: "some-other-namespace",
			},
		}

		match := matchSecretImplicitly(secret, "my-test-namespace")
		require.False(t, match)
	})
	t.Run("do-not-match-with-annotation", func(t *testing.T) {
		secret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "labeled-secret",
				Namespace: "some-other-namespace",
				Annotations: map[string]string{
					types.ReplicatorAnnotationAllowedNamespaces: "-",
				},
			},
		}

		match := matchSecretImplicitly(secret, "my-test-namespace")
		require.False(t, match)
	})
}

func TestMatchSecretByWildcard(t *testing.T) {
	t.Run("match", func(t *testing.T) {
		secret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "labeled-secret",
				Namespace: "some-other-namespace",
				Annotations: map[string]string{
					types.ReplicatorAnnotationAllowedNamespaces: "*",
				},
			},
		}

		match := matchSecretByWildcard(secret, "my-test-namespace")
		require.True(t, match)
	})
	t.Run("do-not-match", func(t *testing.T) {
		secret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "labeled-secret",
				Namespace: "some-other-namespace",
			},
		}

		match := matchSecretByWildcard(secret, "some-other-namespace")
		require.False(t, match)
	})
}

func TestMatchSecretByList(t *testing.T) {
	t.Run("match-single", func(t *testing.T) {
		secret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "labeled-secret",
				Namespace: "some-other-namespace",
				Annotations: map[string]string{
					types.ReplicatorAnnotationAllowedNamespaces: "my-test-namespace",
				},
			},
		}

		match := matchSecretByList(secret, "my-test-namespace")
		require.True(t, match)
	})
	t.Run("do-not-match-single", func(t *testing.T) {
		secret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "labeled-secret",
				Namespace: "some-other-namespace",
				Annotations: map[string]string{
					types.ReplicatorAnnotationAllowedNamespaces: "foo",
				},
			},
		}

		match := matchSecretByList(secret, "my-test-namespace")
		require.False(t, match)
	})
	t.Run("match-list", func(t *testing.T) {
		secret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "labeled-secret",
				Namespace: "some-other-namespace",
				Annotations: map[string]string{
					types.ReplicatorAnnotationAllowedNamespaces: "foo,my-test-namespace,bar",
				},
			},
		}

		match := matchSecretByList(secret, "my-test-namespace")
		require.True(t, match)
	})
	t.Run("do-not-match-list", func(t *testing.T) {
		secret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "labeled-secret",
				Namespace: "some-other-namespace",
				Annotations: map[string]string{
					types.ReplicatorAnnotationAllowedNamespaces: "foo,bar,baz",
				},
			},
		}

		match := matchSecretByList(secret, "my-test-namespace")
		require.False(t, match)
	})
}

func TestGetLabeledSecrets(t *testing.T) {
	t.Run("default-label-selector", func(t *testing.T) {
		_client := testClient.NewSimpleClientset(

			// Matching secrets
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "labeled-secret",
					Namespace: "my-test-namespace",
					Labels: map[string]string{
						types.ReplicatorLabel: "true",
					},
				},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "labeled-pull-secret",
					Namespace: "my-test-namespace",
					Labels: map[string]string{
						types.ReplicatorLabel: "true",
					},
				},
				Type: corev1.SecretTypeDockerConfigJson,
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "labeled-secret-for-current-namespace",
					Namespace: "my-test-namespace",
					Labels: map[string]string{
						types.ReplicatorLabel: "true",
					},
					Annotations: map[string]string{
						types.ReplicatorAnnotationAllowedNamespaces: "-",
					},
				},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "labeled-secret-for-current-namespace-explicitly",
					Namespace: "my-test-namespace",
					Labels: map[string]string{
						types.ReplicatorLabel: "true",
					},
					Annotations: map[string]string{
						types.ReplicatorAnnotationAllowedNamespaces: "my-test-namespace",
					},
				},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "labeled-secret-in-another-namespace-for-any-namespace",
					Namespace: "some-other-namespace",
					Labels: map[string]string{
						types.ReplicatorLabel: "true",
					},
					Annotations: map[string]string{
						types.ReplicatorAnnotationAllowedNamespaces: "*",
					},
				},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "labeled-secret-in-another-namespace-for-this-namespace",
					Namespace: "some-other-namespace",
					Labels: map[string]string{
						types.ReplicatorLabel: "true",
					},
					Annotations: map[string]string{
						types.ReplicatorAnnotationAllowedNamespaces: "my-test-namespace",
					},
				},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "labeled-secret-in-another-namespace-for-many-namespaces",
					Namespace: "some-other-namespace",
					Labels: map[string]string{
						types.ReplicatorLabel: "true",
					},
					Annotations: map[string]string{
						types.ReplicatorAnnotationAllowedNamespaces: "foo,my-test-namespace,bar",
					},
				},
			},

			// Other noise secrets
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-secret",
					Namespace: "my-test-namespace",
				},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-secret",
					Namespace: "some-other-namespace",
				},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "labeled-with-false-secret",
					Namespace: "my-test-namespace",
					Labels: map[string]string{
						types.ReplicatorLabel: "false",
					},
				},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "labeled-secret-for-another-namespace",
					Namespace: "my-test-namespace",
					Labels: map[string]string{
						types.ReplicatorLabel: "true",
					},
					Annotations: map[string]string{
						types.ReplicatorAnnotationAllowedNamespaces: "some-other-namespace",
					},
				},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "labeled-secret-for-many-namespaces-but-not-this-one",
					Namespace: "some-other-namespace",
					Labels: map[string]string{
						types.ReplicatorLabel: "true",
					},
					Annotations: map[string]string{
						types.ReplicatorAnnotationAllowedNamespaces: "foo,bar,baz",
					},
				},
			},
		)

		client := Client{
			_client,
		}

		secrets, err := client.GetLabeledSecrets(context.TODO(), "my-test-namespace", "")
		require.NoError(t, err)

		require.Len(t, secrets.Items, 7)
		secretKeys := make([]string, 0, len(secrets.Items))
		for _, secret := range secrets.Items {
			secretKeys = append(secretKeys, fmt.Sprintf("%s/%s", secret.Namespace, secret.Name))
		}
		require.ElementsMatch(t, []string{
			"my-test-namespace/labeled-secret",
			"my-test-namespace/labeled-pull-secret",
			"my-test-namespace/labeled-secret-for-current-namespace",
			"my-test-namespace/labeled-secret-for-current-namespace-explicitly",
			"some-other-namespace/labeled-secret-in-another-namespace-for-any-namespace",
			"some-other-namespace/labeled-secret-in-another-namespace-for-this-namespace",
			"some-other-namespace/labeled-secret-in-another-namespace-for-many-namespaces",
		}, secretKeys)
	})

	t.Run("alternative-label-selector", func(t *testing.T) {
		_client := testClient.NewSimpleClientset(

			// Matching secrets
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "labeled-secret",
					Namespace: "my-test-namespace",
					Labels: map[string]string{
						types.ReplicatorLabelAlternative: "true",
						"alternative-label":              "alternative-label-value",
					},
				},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "labeled-pull-secret",
					Namespace: "my-test-namespace",
					Labels: map[string]string{
						types.ReplicatorLabelAlternative: "true",
						"alternative-label":              "alternative-label-value",
					},
				},
				Type: corev1.SecretTypeDockerConfigJson,
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "labeled-secret-for-current-namespace",
					Namespace: "my-test-namespace",
					Labels: map[string]string{
						types.ReplicatorLabelAlternative: "true",
						"alternative-label":              "alternative-label-value",
					},
					Annotations: map[string]string{
						types.ReplicatorAnnotationAllowedNamespaces: "-",
					},
				},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "labeled-secret-for-current-namespace-explicitly",
					Namespace: "my-test-namespace",
					Labels: map[string]string{
						types.ReplicatorLabelAlternative: "true",
						"alternative-label":              "alternative-label-value",
					},
					Annotations: map[string]string{
						types.ReplicatorAnnotationAllowedNamespaces: "my-test-namespace",
					},
				},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "labeled-secret-in-another-namespace-for-any-namespace",
					Namespace: "some-other-namespace",
					Labels: map[string]string{
						types.ReplicatorLabelAlternative: "true",
						"alternative-label":              "alternative-label-value",
					},
					Annotations: map[string]string{
						types.ReplicatorAnnotationAllowedNamespaces: "*",
					},
				},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "labeled-secret-in-another-namespace-for-this-namespace",
					Namespace: "some-other-namespace",
					Labels: map[string]string{
						types.ReplicatorLabelAlternative: "true",
						"alternative-label":              "alternative-label-value",
					},
					Annotations: map[string]string{
						types.ReplicatorAnnotationAllowedNamespaces: "my-test-namespace",
					},
				},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "labeled-secret-in-another-namespace-for-many-namespaces",
					Namespace: "some-other-namespace",
					Labels: map[string]string{
						types.ReplicatorLabelAlternative: "true",
						"alternative-label":              "alternative-label-value",
					},
					Annotations: map[string]string{
						types.ReplicatorAnnotationAllowedNamespaces: "foo,my-test-namespace,bar",
					},
				},
			},

			// Other noise secrets
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-secret",
					Namespace: "my-test-namespace",
				},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "labeled-secret-with-default-selector",
					Namespace: "my-test-namespace",
					Labels: map[string]string{
						types.ReplicatorLabel: "true",
					},
					Annotations: map[string]string{
						types.ReplicatorAnnotationAllowedNamespaces: "foo,my-test-namespace,bar",
					},
				},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "labeled-secret-but-not-in-selector",
					Namespace: "some-other-namespace",
					Labels: map[string]string{
						types.ReplicatorLabelAlternative: "true",
						"foo":                            "bar",
					},
					Annotations: map[string]string{
						types.ReplicatorAnnotationAllowedNamespaces: "foo,my-test-namespace,bar",
					},
				},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-secret",
					Namespace: "some-other-namespace",
				},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "labeled-with-false-secret",
					Namespace: "my-test-namespace",
					Labels: map[string]string{
						types.ReplicatorLabel: "false",
					},
				},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "labeled-secret-for-another-namespace",
					Namespace: "my-test-namespace",
					Labels: map[string]string{
						types.ReplicatorLabel: "true",
					},
					Annotations: map[string]string{
						types.ReplicatorAnnotationAllowedNamespaces: "some-other-namespace",
					},
				},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "labeled-secret-for-many-namespaces-but-not-this-one",
					Namespace: "some-other-namespace",
					Labels: map[string]string{
						types.ReplicatorLabel: "true",
					},
					Annotations: map[string]string{
						types.ReplicatorAnnotationAllowedNamespaces: "foo,bar,baz",
					},
				},
			},
		)

		client := Client{
			_client,
		}

		secrets, err := client.GetLabeledSecrets(context.TODO(), "my-test-namespace", "alternative-label=alternative-label-value")
		require.NoError(t, err)

		require.Len(t, secrets.Items, 7)
		secretKeys := make([]string, 0, len(secrets.Items))
		for _, secret := range secrets.Items {
			secretKeys = append(secretKeys, fmt.Sprintf("%s/%s", secret.Namespace, secret.Name))
		}
		require.ElementsMatch(t, []string{
			"my-test-namespace/labeled-secret",
			"my-test-namespace/labeled-pull-secret",
			"my-test-namespace/labeled-secret-for-current-namespace",
			"my-test-namespace/labeled-secret-for-current-namespace-explicitly",
			"some-other-namespace/labeled-secret-in-another-namespace-for-any-namespace",
			"some-other-namespace/labeled-secret-in-another-namespace-for-this-namespace",
			"some-other-namespace/labeled-secret-in-another-namespace-for-many-namespaces",
		}, secretKeys)
	})
}

func TestWriteSecretListManifests(t *testing.T) {
	secrets := &corev1.SecretList{
		Items: []corev1.Secret{
			{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Secret",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-secret",
					Namespace: "some-namespace",
					Labels: map[string]string{
						types.ReplicatorLabel: "true",
						"foo":                 "bar",
					},
					Annotations: map[string]string{
						types.ReplicatorAnnotationAllowedNamespaces: "some-namespace",
						"bar": "baz",
					},
				},
				Data: map[string][]byte{
					"key": []byte("value"),
				},
			},
			{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Secret",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-other-secret",
					Namespace: "some-other-namespace",
					Labels: map[string]string{
						types.ReplicatorLabel: "true",
						"foo":                 "bar",
					},
					Annotations: map[string]string{
						types.ReplicatorAnnotationAllowedNamespaces: "some-namespace",
						"bar": "baz",
					},
				},
				Data: map[string][]byte{
					"key": []byte("value"),
				},
			},
			{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Secret",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret-with-replicated-name",
					Namespace: "some-namespace",
					Annotations: map[string]string{
						types.ReplicatorAnnotationReplicatedName: "replicated-secret",
					},
				},
				Data: map[string][]byte{
					"key": []byte("value"),
				},
			},
		},
	}

	buf := bytes.NewBufferString("")
	client := Client{nil}
	client.WriteSecretListManifests(context.TODO(), "my-test-namespace", secrets, buf)

	require.Equal(t, secretsYAML, buf.String())
}
