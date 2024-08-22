# tokenaut: GitHub App Installation Access Token Controller

[![Docker Repository on Quay](https://quay.io/repository/appthrust/tokenaut/status "Docker Repository on Quay")](https://quay.io/repository/appthrust/tokenaut)

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

## Secret Deletion

When an InstallationAccessToken is deleted, the associated Secret is automatically deleted as well. This ensures that no orphaned Secrets are left in the cluster after an InstallationAccessToken is removed.

The deletion process follows these steps:

1. When an InstallationAccessToken is marked for deletion, the controller initiates the cleanup process.
2. The controller attempts to delete the associated Secret, as specified in the InstallationAccessToken's status.
3. If the Secret deletion is successful or the Secret is not found (possibly already deleted), the controller proceeds with removing the InstallationAccessToken.
4. If there's an error during the Secret deletion (other than "not found"), the controller will retry the operation.

This automatic cleanup ensures that your cluster remains tidy and that sensitive information (the access token) is properly removed when it's no longer needed.

Note: Ensure that the controller has the necessary permissions to delete Secrets in the relevant namespaces. If you're using InstallationAccessTokens across different namespaces, you may need to adjust your RBAC settings accordingly.

## Secret Metadata

To improve the manageability and traceability of Secrets created by tokenaut, we've implemented additional metadata for these Secrets. This metadata helps operators easily identify which Secrets are managed by tokenaut and track their relationship to InstallationAccessTokens.

### Labels

Each Secret created by tokenaut includes the following labels:

- `app.kubernetes.io/managed-by: tokenaut`: Indicates that this Secret is managed by tokenaut.
- `tokenaut.appthrust.io/installation-access-token: <namespace>.<name>`: Identifies the specific InstallationAccessToken resource that this Secret is associated with, including its namespace to ensure uniqueness across the cluster.

### Annotations

The following annotations are added to each Secret:

- `tokenaut.appthrust.io/last-updated`: Timestamp of when the Secret was last updated.
- `tokenaut.appthrust.io/app-id`: The GitHub App ID associated with this Secret.
- `tokenaut.appthrust.io/installation-id`: The GitHub App Installation ID associated with this Secret.
- `tokenaut.appthrust.io/source-namespace`: The namespace of the source InstallationAccessToken.
- `tokenaut.appthrust.io/source-name`: The name of the source InstallationAccessToken.

### Use Cases

These metadata additions enable several useful operations:

1. List all Secrets managed by tokenaut:
	 ```
	 kubectl get secrets -l app.kubernetes.io/managed-by=tokenaut
	 ```

2. Find the Secret associated with a specific InstallationAccessToken:
	 ```
	 kubectl get secrets -l tokenaut.appthrust.io/installation-access-token=<namespace>.<name>
	 ```

3. View detailed information about a Secret, including its associated GitHub App and InstallationAccessToken:
	 ```
	 kubectl describe secret <secret-name>
	 ```

4. Find all Secrets associated with a specific GitHub App:
	 ```
	 kubectl get secrets -o json | jq '.items[] | select(.metadata.annotations."tokenaut.appthrust.io/app-id"=="<app-id>")'
	 ```

5. Find all Secrets from a specific namespace's InstallationAccessTokens:
	 ```
	 kubectl get secrets -o json | jq '.items[] | select(.metadata.annotations."tokenaut.appthrust.io/source-namespace"=="<namespace>")'
	 ```

These metadata additions make it easier for operators to manage and track the Secrets created by tokenaut, enhancing the overall observability and maintainability of the system.

## Best Practices

### Annotating the GitHub App Private Key Secret

When creating the `github-app-private-key` Secret, it's beneficial to include additional metadata as annotations. While the `privateKey` field is the only required data, adding extra information can greatly improve key management and traceability. Consider including the following annotations:

- `sha256`: The SHA256 fingerprint of the private key
- `created-at`: The creation date of the private key
- `github-app-url`: The URL of the GitHub App

Here's an example of how to create the Secret with these annotations:

```diff
 apiVersion: v1
 kind: Secret
 metadata:
   name: github-app-private-key
   namespace: default
+  annotations:
+    example.com/sha256: 6Bh3506/pnTDWJ/YxCU22p5RZgx7NDvoPfy7UMEXsJ8=
+    example.com/created-at: 2024-04-01T12:00:00Z
+    example.com/github-app-url: https://github.com/organizations/your-org/settings/apps/your-app-slug
 type: tokenaut.appthrust.io/private-key
 stringData:
   privateKey: |
     -----BEGIN RSA PRIVATE KEY-----
     ...
     -----END RSA PRIVATE KEY-----
```

Benefits of this approach:

1. **Easy Identification**: The SHA256 fingerprint is displayed in the GitHub UI, making it easy to match the Secret with the correct key in your GitHub App settings.
2. **Auditing**: The creation date helps track when the key was generated, useful for key rotation policies and auditing.
3. **Simplified Maintenance**: These annotations make it easier to manage multiple keys or troubleshoot issues related to key expiration or mismatch.
4. **Clear Association**: The GitHub App URL provides a direct link to the associated app, eliminating any ambiguity about which app the key belongs to.

By following this practice, you can significantly improve the manageability and traceability of your GitHub App private keys within your Kubernetes cluster. The added GitHub App URL annotation ensures that you can quickly navigate to the correct app settings, which is particularly helpful when managing multiple GitHub Apps or in large organizations.

## Troubleshooting

### Error: "Failed to create token: unexpected status code: 401: A JSON web token could not be decoded"

If you encounter this error, it typically means that the GitHub App's private key is incorrect, and GitHub is rejecting the attempt to issue a token with an invalid JWT.

To resolve this issue:

1. Verify that the GitHub App's private key is correctly stored in the Secret resource.
2. Double-check that you've copied the entire private key, including the `-----BEGIN RSA PRIVATE KEY-----` and `-----END RSA PRIVATE KEY-----` lines.
3. If the problem persists, try regenerating a new private key for your GitHub App and update the Secret accordingly.

Remember to always keep your private key secure and never expose it in your code or version control systems.
