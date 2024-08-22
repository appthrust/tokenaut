# tokenaut: GitHub App Installation Access Token Controller

tokenaut is a controller for managing GitHub App Installation Access Tokens (server-to-server tokens, ghs).

## Problem Solved

GitHub Apps issue relatively short-lived Installation Access Tokens. These tokens are often needed for tools like ArgoCD, FluxCD, or Crossplane provider-github. (While ArgoCD and provider-github support GitHub App private keys, you might prefer not to distribute private keys to various pods.) In such cases, it's necessary to maintain an active token that's always within its validity period. This project aims to solve this challenge using the Kubernetes controller pattern.

## Controller's Role

The controller periodically recreates the GitHub token contained in the Secret resource to prevent it from expiring, keeping the Secret resource up-to-date.

![](docs/assets/fig1.png)

## Mechanism

A Secret resource is created based on the InstallationAccessToken resource. When there's a change to the InstallationAccessToken resource, the Secret resource is updated.

![](docs/assets/fig2.png)

To operate this controller, you need the GitHub App's private key. First, create a Secret containing this key with the name "github-app-private-key":

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: github-app-private-key
  namespace: default
type: tokenaut.appthrust.io/private-key
stringData:
  privateKey: |
    -----BEGIN RSA PRIVATE KEY-----
    ...
    -----END RSA PRIVATE KEY-----
```

> NOTE: The name and namespace can be customized using the "Explicit Private Key" method described later. This allows handling multiple private keys or using preferred names.

Next, create the InstallationAccessToken resource, which is the source for the Secret. This resource requires at least two fields: appId and installationId, specifying which GitHub App and which installation to use.

```yaml
apiVersion: tokenaut.appthrust.io/v1alpha1
kind: InstallationAccessToken
metadata:
  name: our-github-token
  namespace: default
spec:
  appId: "12345"
  installationId: "1234567890"
```

When the controller discovers this InstallationAccessToken resource, it creates a Secret like this:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: our-github-token
  namespace: default
type: Opaque
stringData:
  token: ghs_16C7e42F292c6912E7710c838347Ae178B4a
```

By default, the created Secret follows these simple rules:

- `metadata.name`: Inherited from the InstallationAccessToken's `metadata.name`.
- `metadata.namespace`: Inherited from the InstallationAccessToken's `metadata.namespace`. The Secret is created in the same namespace.
- `data.token`: Contains the Base64 encoded token directly.

> NOTE: This behavior can be modified using the "Custom Secret" method described below.

## Explicit Private Key

By default, the controller looks for a Secret named "github-app-private-key" in the "default" namespace and tries to recognize its "privateKey" field as the private key.

You can explicitly specify the private key if you want to:

- Use a different `metadata.name` for the private key Secret.
- Use a namespace other than "default".
- Use a field name other than "privateKey".
- Use multiple private keys.

To explicitly specify the private key, set `spec.privateKeyRef` in the InstallationAccessToken. For example, to use a Secret named "my-key":

```diff
 apiVersion: tokenaut.appthrust.io/v1alpha1
 kind: InstallationAccessToken
 metadata:
   name: our-github-token
   namespace: default
 spec:
   appId: "12345"
   installationId: "1234567890"
+  privateKeyRef:
+    name: my-key
```

To specify the namespace and field name as well:

```diff
 apiVersion: tokenaut.appthrust.io/v1alpha1
 kind: InstallationAccessToken
 metadata:
   name: our-github-token
   namespace: default
 spec:
   appId: "12345"
   installationId: "1234567890"
+  privateKeyRef:
+    name: my-key
+    namespace: my-space
+    key: pem
```

This specification will look for a Secret with the following structure:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: my-key
  namespace: my-space
type: tokenaut.appthrust.io/private-key
stringData:
  pem: ...snip...
```

## Custom Secret

When converting InstallationAccessToken to Secret, the controller follows these rules:

1. Copy `metadata.name` from InstallationAccessToken to Secret's `metadata.name`.
2. Copy `metadata.namespace` from InstallationAccessToken to Secret's `metadata.namespace`.
3. Write the Base64 encoded token to Secret's `data.token`.

This default behavior can be changed using `spec.template` in the InstallationAccessToken.

For example, to store the token in a "password" field:

```diff
 apiVersion: tokenaut.appthrust.io/v1alpha1
 kind: InstallationAccessToken
 metadata:
   name: our-github-token
   namespace: default
 spec:
   appId: "12345"
   installationId: "1234567890"
+  template:
+    data:
+      password: "{{ .Token }}"
```

This generates a Secret like:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: our-github-token
  namespace: default
type: Opaque
data:
  password: "(Base64 encoded token)"
```

To change the name or namespace, specify them in the template:

```diff
 apiVersion: tokenaut.appthrust.io/v1alpha1
 kind: InstallationAccessToken
 metadata:
   name: our-github-token
   namespace: default
 spec:
   appId: "12345"
   installationId: "1234567890"
   template:
+    metadata:
+      name: custom-secret-name
+      namespace: custom-namespace
     data:
       password: "{{ .Token }}"
```

This generates:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: custom-secret-name
  namespace: custom-namespace
type: Opaque
data:
  password: "(Base64 encoded token)"
```

The template can specify any field of a Secret object, including `type`:

```diff
 apiVersion: tokenaut.appthrust.io/v1alpha1
 kind: InstallationAccessToken
 metadata:
   name: our-github-token
   namespace: default
 spec:
   appId: "12345"
   installationId: "1234567890"
   template:
     metadata:
       name: custom-secret-name
       namespace: custom-namespace
+    type: my-custom-type
     data:
       password: "{{ .Token }}"
```

Sometimes you might want to handle a string with an embedded token, such as a URL for `git clone`:

```
https://ghs_16C7e42F292c6912E7710c838347Ae178B4a@github.com/my-org/my-repo.git
```

You can generate such a URL using `{{ .Token }}` for string interpolation:

```diff
 apiVersion: tokenaut.appthrust.io/v1alpha1
 kind: InstallationAccessToken
 metadata:
   name: our-github-token
   namespace: default
 spec:
   appId: "12345"
   installationId: "1234567890"
   template:
     stringData:
+      cloneUrl: "https://{{ .Token }}@github.com/my-org/my-repo.git"
```

## Access Token Scope

> NOTE: This feature is a future idea that may be implemented.

By default, the created access tokens have no scope settings. If you want to narrow the scope of access tokens by repository or permissions, you can specify this in `spec.scope` of the InstallationAccessToken.

```diff
 apiVersion: tokenaut.appthrust.io/v1alpha1
 kind: InstallationAccessToken
 metadata:
   name: our-github-token
   namespace: default
 spec:
   appId: "12345"
   installationId: "1234567890"
+  scope:
+    repositories:
+      - repo1
+      - repo2
+    permissions:
+      contents: write
+      metadata: read
```

The configurable scopes are as follows:

| Item | Description | Data Type |
| --- | --- | --- |
| repositories | Repository names | string[] |
| repositoryIds | Repository IDs | int[] |
| permissions | Permissions. For settable permissions, refer to: [GitHub Docs](https://docs.github.com/en/rest/apps/apps?apiVersion=2022-11-28#create-an-installation-access-token-for-an-app) | map[string]string |

## Duplicate Token Elimination

> NOTE: This feature is a future idea that may be implemented.

GitHub App installation access tokens have a limit of 10 per hour for each combination of user * app * scope. If this limit is exceeded, the oldest token is revoked. Therefore, it's desirable not to create duplicate tokens with the same role.

To solve this limitation, if a token with the same combination already exists, it will be reused. The token for reuse is stored as a normal Secret. As a marker, the `type` is set to `tokenaut.appthrust.io/access-token-cache`.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: github-app-access-token-{random}
  namespace: default
  annotations:
    tokenaut.appthrust.io/repositories: '["foo","bar"]'
    tokenaut.appthrust.io/repository-ids: '[123,456]'
    tokenaut.appthrust.io/permissions: '{"contents":"write","metadata":"read"}'
type: tokenaut.appthrust.io/access-token-cache
stringData:
  token: "ghs_16C7e42F292c6912E7710c838347Ae178B4a"
```

> NOTE: This design guarantees duplicate elimination at the cluster level. It does not consider eliminating duplicates across multiple clusters or between a cluster and non-Kubernetes systems. This design may be revisited from scratch.

## Token Refresh Frequency

GitHub App installation access tokens have a lifespan of 1 hour. The controller refreshes all tokens at a frequency of 50 minutes. This is to ensure that the token is always valid and to avoid the token becoming invalid during the update process. If you need to change the update frequency, you can specify `-token-refresh-interval` in the controller's command line arguments.

## Manual Trigger for Token Update

You might want to update a token manually without waiting for an hour. In such cases, you can prompt the controller to update by making a change to the InstallationAccessToken object, such as changing its `metadata.annotations`.

## Status

The `status` of an InstallationAccessToken includes the following information, which can be used for operational reference or automation:

```yaml
status:
  conditions:
    - type: Token
      status: "True"
      reason: Created
      message: "Token successfully created"
      lastTransitionTime: "2023-04-01T12:00:00Z"
    - type: Secret
      status: "True"
      reason: Updated
      message: "Secret successfully created/updated"
      lastTransitionTime: "2023-04-01T12:00:05Z"
    - type: Ready
      status: "True"
      reason: AllReady
      message: "InstallationAccessToken is ready for use"
      lastTransitionTime: "2023-04-01T12:00:05Z"
  secretRef:
    name: "our-github-token"
    namespace: "default"
  token:
    expiresAt: "2023-04-01T13:00:00Z"
    permissions:
      issues: "write"
      contents: "read"
    repositorySelection: "selected"
    repositories:
      - "octocat/Hello-World"
    repositoryIds:
      - 1296269
```

### Conditions

**type=Token**

| Status | Reason | Message | Description |
| --- | --- | --- | --- |
| True | Created | Token successfully created | Token was successfully created |
| False | Failed | Failed to create token: {error_message} | Failed to create token. Includes error message |
| Unknown | Pending | Token creation in progress | Token creation is in progress |

**type=Secret**

| Status | Reason | Message | Description |
| --- | --- | --- | --- |
| True | Updated | Secret successfully created/updated | Secret resource was successfully created or updated |
| False | Failed | Failed to create/update Secret: {error_message} | Failed to create or update Secret resource. Includes error message |
| Unknown | Pending | Secret creation/update in progress | Secret resource creation or update is in progress |

**type=Ready**

| Status | Reason | Message | Description |
| --- | --- | --- | --- |
| True | AllReady | InstallationAccessToken is ready for use | Token has been generated and Secret resource has been successfully created/updated |
| False | TokenNotReady | Token is not ready: {reason} | Token is not in a usable state. Includes reason |
| False | SecretNotReady | Secret is not ready: {reason} | Secret resource is not in a usable state. Includes reason |
| False | InvalidConfiguration | Invalid configuration: {details} | Resource configuration is invalid. Includes details |
| Unknown | Pending | Resource reconciliation in progress | Resource reconciliation is in progress |

