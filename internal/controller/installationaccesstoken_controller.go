package controller

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"html/template"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	tokenautv1alpha1 "github.com/appthrust/tokenaut/api/v1alpha1"
	"github.com/appthrust/tokenaut/pkg/githubapi"
	"github.com/appthrust/tokenaut/pkg/githubappjwt"
)

const (
	// FinalizerName is the name of the finalizer used to clean up secrets
	FinalizerName = "tokenaut.appthrust.io/cleanup-secret"
)

// InstallationAccessTokenReconciler reconciles a InstallationAccessToken object
type InstallationAccessTokenReconciler struct {
	client.Client
	Scheme               *runtime.Scheme
	TokenRefreshInterval time.Duration
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *InstallationAccessTokenReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the InstallationAccessToken instance
	var installationAccessToken tokenautv1alpha1.InstallationAccessToken
	if err := r.Get(ctx, req.NamespacedName, &installationAccessToken); err != nil {
		if apierrors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}
	log.Info("Reconciling InstallationAccessToken", "Generation", installationAccessToken.Generation)

	// Add finalizer if it doesn't exist
	if !controllerutil.ContainsFinalizer(&installationAccessToken, FinalizerName) {
		controllerutil.AddFinalizer(&installationAccessToken, FinalizerName)
		if err := r.Update(ctx, &installationAccessToken); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Check if the InstallationAccessToken is being deleted
	if !installationAccessToken.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, &installationAccessToken)
	}

	// Get the private key
	privateKey, err := r.getPrivateKey(ctx, &installationAccessToken)
	if err != nil {
		log.Error(err, "Failed to get private key")
		return r.updateStatusWithError(ctx, &installationAccessToken, "InvalidConfiguration", err)
	}

	// Generate JWT
	jwt, err := githubappjwt.Generate(installationAccessToken.Spec.AppID, privateKey)
	if err != nil {
		log.Error(err, "Failed to generate JWT")
		return r.updateStatusWithError(ctx, &installationAccessToken, "JWTGenerationError", err)
	}

	// Create GitHub API client
	githubClient := githubapi.NewClient(githubapi.ClientConfig{})

	// Create installation access token
	log.Info("Creating installation access token", "InstallationID", installationAccessToken.Spec.InstallationID)
	tokenResp, err := githubClient.CreateInstallationAccessToken(installationAccessToken.Spec.InstallationID, jwt)
	if err != nil {
		log.Error(err, "Failed to create installation access token")
		return r.updateStatusWithError(ctx, &installationAccessToken, "TokenCreationError", err)
	}

	// Update Token condition
	r.updateTokenCondition(ctx, &installationAccessToken, tokenResp, nil)

	// Create or update the Secret
	createdSecret, err := r.createOrUpdateSecret(ctx, &installationAccessToken, tokenResp.Token)
	if err != nil {
		log.Error(err, "Failed to create or update secret")
		return r.updateStatusWithError(ctx, &installationAccessToken, "SecretUpdateError", err)
	}

	// Update Secret condition
	r.updateSecretCondition(ctx, &installationAccessToken, createdSecret, nil)

	// Update overall status
	r.updateOverallStatus(ctx, &installationAccessToken)

	// Requeue after 50 minutes to refresh the token before it expires
	log.Info(fmt.Sprintf("Completed reconciliation for %s, requeuing after %v", req.NamespacedName, r.TokenRefreshInterval))
	return ctrl.Result{RequeueAfter: r.TokenRefreshInterval}, nil
}

func (r *InstallationAccessTokenReconciler) getPrivateKey(ctx context.Context, iat *tokenautv1alpha1.InstallationAccessToken) (*rsa.PrivateKey, error) {
	secretName := "github-app-private-key"
	secretNamespace := "default"
	secretKey := "privateKey"

	if iat.Spec.PrivateKeyRef != nil {
		if iat.Spec.PrivateKeyRef.Name != "" {
			secretName = iat.Spec.PrivateKeyRef.Name
		}
		if iat.Spec.PrivateKeyRef.Namespace != "" {
			secretNamespace = iat.Spec.PrivateKeyRef.Namespace
		}
		if iat.Spec.PrivateKeyRef.Key != "" {
			secretKey = iat.Spec.PrivateKeyRef.Key
		}
	}

	var secret corev1.Secret
	if err := r.Get(ctx, types.NamespacedName{Name: secretName, Namespace: secretNamespace}, &secret); err != nil {
		return nil, errors.Errorf("tried to get a secret named \"%s\" in namespace \"%s\", but got error: %v. Please create the secret with the private key or specify the correct secret name and key in the InstallationAccessToken `spec.privateKeyRef`", secretName, secretNamespace, err)
	}

	privateKeyPEM, ok := secret.Data[secretKey]
	if !ok {
		return nil, errors.Errorf("tried to read the key \"%s\" from the secret \"%s\" in namespace \"%s\", but the key was not found. Please create the secret with the private key or specify the correct secret name and key in the InstallationAccessToken `spec.privateKeyRef`", secretKey, secretName, secretNamespace)
	}

	block, _ := pem.Decode(privateKeyPEM)
	if block == nil {
		return nil, errors.Errorf("failed to parse PEM block containing the private key")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, errors.Errorf("failed to parse private key: %v", err)
	}

	return privateKey, nil
}

func (r *InstallationAccessTokenReconciler) createOrUpdateSecret(ctx context.Context, iat *tokenautv1alpha1.InstallationAccessToken, token string) (*corev1.Secret, error) {
	namespace := iat.Namespace
	name := iat.Name
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			"token": token,
		},
	}

	if iat.Spec.Template != nil {
		var templateData map[string]interface{}
		if err := json.Unmarshal(iat.Spec.Template.Raw, &templateData); err != nil {
			return nil, errors.Errorf("failed to unmarshal template: %v", err)
		}

		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(templateData, secret); err != nil {
			return nil, errors.Errorf("failed to apply template: %v", err)
		}

		// Ensure the token is still present in the secret data
		if secret.StringData == nil {
			secret.StringData = make(map[string]string)
		}
		for k, v := range secret.StringData {
			tpl, err := template.New("secret").Parse(v)
			if err != nil {
				return nil, errors.Errorf("failed to parse template: %v", err)
			}
			out := new(strings.Builder)
			err = tpl.Execute(out, map[string]string{"Token": token})
			if err != nil {
				return nil, errors.Errorf("failed to execute template: %v", err)
			}
			secret.StringData[k] = out.String()
		}
	}

	// Add metadata to the secret
	if secret.Labels == nil {
		secret.Labels = make(map[string]string)
	}
	secret.Labels["app.kubernetes.io/managed-by"] = "tokenaut"
	secret.Labels["tokenaut.appthrust.io/installation-access-token"] = fmt.Sprintf("%s.%s", iat.Namespace, iat.Name)

	if secret.Annotations == nil {
		secret.Annotations = make(map[string]string)
	}
	secret.Annotations["tokenaut.appthrust.io/last-updated"] = time.Now().Format(time.RFC3339)
	secret.Annotations["tokenaut.appthrust.io/app-id"] = iat.Spec.AppID
	secret.Annotations["tokenaut.appthrust.io/installation-id"] = iat.Spec.InstallationID
	secret.Annotations["tokenaut.appthrust.io/source-namespace"] = iat.Namespace
	secret.Annotations["tokenaut.appthrust.io/source-name"] = iat.Name

	// Create or update the secret
	err := r.Create(ctx, secret)
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			// Secret already exists, update it
			err = r.Update(ctx, secret)
			if err != nil {
				return nil, errors.Errorf("failed to update secret: %v", err)
			}
		} else {
			return nil, errors.Errorf("failed to create secret: %v", err)
		}
	}

	return secret, nil
}

func (r *InstallationAccessTokenReconciler) updateStatusWithError(ctx context.Context, iat *tokenautv1alpha1.InstallationAccessToken, reason string, err error) (ctrl.Result, error) {
	r.updateTokenCondition(ctx, iat, nil, err)
	r.updateSecretCondition(ctx, iat, nil, err)
	r.updateOverallStatus(ctx, iat)

	return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
}

func (r *InstallationAccessTokenReconciler) updateTokenCondition(ctx context.Context, iat *tokenautv1alpha1.InstallationAccessToken, tokenResp *githubapi.AccessTokenResponse, err error) {
	condition := metav1.Condition{
		Type:               "Token",
		LastTransitionTime: metav1.Now(),
	}

	if tokenResp != nil {
		condition.Status = metav1.ConditionTrue
		condition.Reason = "Created"
		condition.Message = "Token successfully created"
		iat.Status.Token = tokenautv1alpha1.TokenInfo{
			ExpiresAt:           metav1.NewTime(tokenResp.ExpiresAt),
			Permissions:         tokenResp.Permissions,
			RepositorySelection: tokenResp.RepositorySelection,
			// TODO: Uncomment these lines when the API is updated
			//Repositories:        tokenResp.Repositories,
			//RepositoryIDs:       tokenResp.RepositoryIDs,
		}
	} else {
		condition.Status = metav1.ConditionFalse
		condition.Reason = "Failed"
		if err != nil {
			condition.Message = fmt.Sprintf("Failed to create token: %v", err)
		} else {
			condition.Message = "Failed to create token"
		}
	}

	meta.SetStatusCondition(&iat.Status.Conditions, condition)
}

func (r *InstallationAccessTokenReconciler) updateSecretCondition(ctx context.Context, iat *tokenautv1alpha1.InstallationAccessToken, createdSecret *corev1.Secret, err error) {
	condition := metav1.Condition{
		Type:               "Secret",
		LastTransitionTime: metav1.Now(),
	}

	if createdSecret != nil {
		condition.Status = metav1.ConditionTrue
		condition.Reason = "Updated"
		condition.Message = "Secret successfully created/updated"
		iat.Status.SecretRef = tokenautv1alpha1.SecretRef{
			Name:      createdSecret.Name,
			Namespace: createdSecret.Namespace,
		}
	} else {
		condition.Status = metav1.ConditionFalse
		condition.Reason = "Failed"
		if err != nil {
			condition.Message = fmt.Sprintf("Failed to create/update Secret: %v", err)
		} else {
			condition.Message = "Failed to create/update Secret: unknown error"
		}
	}

	meta.SetStatusCondition(&iat.Status.Conditions, condition)
}

func (r *InstallationAccessTokenReconciler) updateOverallStatus(ctx context.Context, iat *tokenautv1alpha1.InstallationAccessToken) {
	tokenCondition := meta.FindStatusCondition(iat.Status.Conditions, "Token")
	secretCondition := meta.FindStatusCondition(iat.Status.Conditions, "Secret")

	condition := metav1.Condition{
		Type:               "Ready",
		LastTransitionTime: metav1.Now(),
	}

	if tokenCondition != nil && secretCondition != nil &&
		tokenCondition.Status == metav1.ConditionTrue &&
		secretCondition.Status == metav1.ConditionTrue {
		condition.Status = metav1.ConditionTrue
		condition.Reason = "AllReady"
		condition.Message = "InstallationAccessToken is ready for use"
	} else {
		condition.Status = metav1.ConditionFalse
		condition.Reason = "NotReady"
		if tokenCondition != nil && tokenCondition.Status == metav1.ConditionFalse {
			condition.Message = fmt.Sprintf("Token is not ready: %s", tokenCondition.Message)
		} else if secretCondition != nil && secretCondition.Status == metav1.ConditionFalse {
			condition.Message = fmt.Sprintf("Secret is not ready: %s", secretCondition.Message)
		} else {
			condition.Message = "InstallationAccessToken is not ready"
		}
	}

	meta.SetStatusCondition(&iat.Status.Conditions, condition)

	if err := r.Status().Update(ctx, iat); err != nil {
		log.FromContext(ctx).Error(err, "Failed to update InstallationAccessToken status")
	}
}

func (r *InstallationAccessTokenReconciler) reconcileDelete(ctx context.Context, iat *tokenautv1alpha1.InstallationAccessToken) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Starting deletion process for InstallationAccessToken",
		"name", iat.Name,
		"namespace", iat.Namespace)
	if iat.Status.SecretRef.Name != "" && iat.Status.SecretRef.Namespace != "" {
		secretToDelete := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      iat.Status.SecretRef.Name,
				Namespace: iat.Status.SecretRef.Namespace,
			},
		}
		if err := r.Delete(ctx, secretToDelete); err != nil && !apierrors.IsNotFound(err) {
			log.Error(err, "Failed to delete associated Secret",
				"secretName", secretToDelete.Name,
				"secretNamespace", secretToDelete.Namespace)
			return ctrl.Result{RequeueAfter: time.Second * 10}, err
		}
	}
	controllerutil.RemoveFinalizer(iat, FinalizerName)
	if err := r.Update(ctx, iat); err != nil {
		log.Error(err, "Failed to remove finalizer from InstallationAccessToken")
		return ctrl.Result{}, err
	}
	log.Info("Successfully completed deletion process for InstallationAccessToken",
		"name", iat.Name,
		"namespace", iat.Namespace)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *InstallationAccessTokenReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tokenautv1alpha1.InstallationAccessToken{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Complete(r)
}
