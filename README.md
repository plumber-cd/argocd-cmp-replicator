# argocd-cmp-replicator

This is a tool that can be used as ArgoCD Config Management Plugin.

See https://argo-cd.readthedocs.io/en/stable/operator-manual/config-management-plugins/.

It is useful in a multi-cluster environment where ArgoCD is deployed in a central cluster, and you need to replicate the same secrets to all clusters managed by it. It may be some common pull secrets, CA certificates etc. This will not allow you to replicate secrets within the same cluster to multiple namespaces (other than the local ArgoCD cluster).

It can find all secrets in the local ArgoCD cluster labeled with `plumber-cd.github.io/argocd-cmp-replicator=true` and add them to the desired state for your ArgoCD Application.

Effectively, it allows you to replicate secrets from one cluster to another without any external secret management tool or operator with cross-cluster access - just by using ArgoCD.

## Deployment

Create a role for the plugin that would allow it to read all secrets in the local cluster:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: argocd-cmp-replicator
rules:
- apiGroups:
  - ""
  resources:
  - secrets
  verbs:
  - get
  - list
```

Bind it to the ArgoCD Repo Server service account:

> :warning: **Careful with `automountServiceAccountToken: true`, you must inspect any other side cars that could be mounting the token as they all will potentially get access to all the secrets in the cluster**

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: argocd-cmp-replicator
roleRef:
    apiGroup: rbac.authorization.k8s.io
    kind: ClusterRole
    name: argocd-cmp-replicator
subjects:
- kind: ServiceAccount
  name: argocd-repo-server
  namespace: argocd
```

Patch ArgoCD Repo Server to add this plugin as a sidecar:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: argocd-repo-server
spec:
  template:
    spec:
      containers:
      - name: argocd-cmp-replicator
        command: [/var/run/argocd/argocd-cmp-server]
        image: ghcr.io/plumber-cd/argocd-cmp-replicator:latest
        securityContext:
          runAsNonRoot: true
          runAsUser: 999
        volumeMounts:
          - mountPath: /var/run/argocd
            name: var-files
          - mountPath: /home/argocd/cmp-server/plugins
            name: plugins
          - mountPath: /tmp
            name: argocd-cmp-replicator-tmp
          - name: service-account-token
          mountPath: "/var/run/secrets/kubernetes.io/serviceaccount"
          readOnly: true
      volumes:
      - name: argocd-cmp-replicator-tmp
        emptyDir: {}
```

Lastly, your Application needs to use the plugin (`repoURL`, `targetRevision` and `path` are not really used, but changes to these locations will trigger ArgoCD Application Refreshes, so it is a good idea to set them to something that will not change very often):

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: replicated-secrets
  namespace: argocd
spec:
  source:
    repoURL: https://github.com/foo/bar
    targetRevision: main
    path: .
    plugin:
      name: argocd-cmp-replicator
  destination:
    name: in-cluster
    namespace: my-test-namespace
  syncPolicy:
    syncOptions:
    # In privileged projects, you will always want to use this in combination with this plugin to avoid potential conflicts
    - FailOnSharedResource=true
```

## Usage

To allow the secret to be replicated, label it with `plumber-cd.github.io/argocd-cmp-replicator=true`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: my-secret
  labels:
    plumber-cd.github.io/argocd-cmp-replicator: "true"
```

By default, it will be allowed to replicate into any cluster as long as the Application `.spec.destination.namespace` is set to the same namespace as the secret is in. If you want to allow the secret to replicate to a different namespace, you can add an annotation to the secret:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: my-secret
  labels:
    plumber-cd.github.io/argocd-cmp-replicator: "true"
  annotations:
    plumber-cd.github.io/argocd-cmp-replicator-allowed-namespaces: "my-test-namespace"
```

Special value `-` (single dash) means the same as not setting the annotation at all, i.e. the secret will be allowed to replicate to the same namespace only.

You can also specify multiple namespaces:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: my-secret
  labels:
    plumber-cd.github.io/argocd-cmp-replicator: "true"
  annotations:
    plumber-cd.github.io/argocd-cmp-replicator-allowed-namespaces: "my-test-namespace,my-other-namespace"
```

Finally, you can allow it to replicate to any namespace by setting the annotation to `*`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: my-secret
  labels:
    plumber-cd.github.io/argocd-cmp-replicator: "true"
  annotations:
    plumber-cd.github.io/argocd-cmp-replicator-allowed-namespaces: "*"
```

By default replicated secret name will be `{{ .originalSecret.Name }}-from-{{ .originalSecret.Namespace }}` to avoid any potential naming conflicts with existing secrets. To change that behavior, you can use annotation `plumber-cd.github.io/argocd-cmp-replicator-replicated-name`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: my-secret
  labels:
    plumber-cd.github.io/argocd-cmp-replicator: "true"
  annotations:
    plumber-cd.github.io/argocd-cmp-replicator-allowed-namespaces: "*"
    plumber-cd.github.io/argocd-cmp-replicator-replicated-name: default-pull-secret
```

Note that in privileged projects (that are allowed to sync to multiple namespaces) you will always want to setsync policy `FailOnSharedResource=true`. Otherwise, user in a namespace A could override a secret in a namespace B. In user-projects bound to specific namespaces, this CMP will produce conflicting intent, but ArgoCD will refuse to sync it to a namespace not listed on the project. In future, we may add annotation on the namespace that would establish trust from other namespaces to avoid this conflict altogether.

### Non-standard label selector

By default, the plugin will look for secrets with the label `plumber-cd.github.io/argocd-cmp-replicator=true`. If you want to use a different label, which may be useful in a multi-tenant clusters, you can label secrets with alternative label `plumber-cd.github.io/argocd-cmp-replicator-use-alternative-selector=true` and a set of additional labels that you want to use:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: my-secret
  labels:
    plumber-cd.github.io/argocd-cmp-replicator-use-alternative-selector=true: "true"
    my-other-label: "value"
    one-more-label: "another-value"
```

Additional labels are specified on the Application:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: replicated-secrets
  namespace: argocd
spec:
  source:
    repoURL: https://github.com/foo/bar
    targetRevision: main
    path: .
    plugin:
      name: argocd-cmp-replicator
      parameters:
        - name: alternative-label-selector
          string: 'my-other-label=value,one-more-label=another-value'
  destination:
    name: in-cluster
    namespace: my-test-namespace
  syncPolicy:
    syncOptions:
    # You will always want to use this in combination with this plugin to avoid potential conflicts
    - FailOnSharedResource=true
```
