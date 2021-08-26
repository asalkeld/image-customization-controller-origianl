/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"errors"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/asalkeld/image-customization-controller/pkg/imagehandler"
	metal3 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	"github.com/metal3-io/baremetal-operator/pkg/secretutils"
)

const (
	minRetryDelay = time.Second * 10
	maxRetryDelay = time.Minute * 10
)

// PreprovisioningImageReconciler reconciles a PreprovisioningImage object
type PreprovisioningImageReconciler struct {
	client.Client
	Log             logr.Logger
	Scheme          *runtime.Scheme
	APIReader       client.Reader
	ImageFileServer imagehandler.ImageFileServer
}

type conditionReason string

const (
	reasonSuccess            conditionReason = "ImageSuccess"
	reasonConfigurationError conditionReason = "ConfigurationError"
	reasonMissingNetworkData conditionReason = "MissingNetworkData"
	reasonUnexpectedError    conditionReason = "UnexpectedError"
	reasonImageServingError  conditionReason = "ImageServingError"
)

// +kubebuilder:rbac:groups=metal3.io,resources=preprovisioningimages,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=metal3.io,resources=preprovisioningimages/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;update

func (r *PreprovisioningImageReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	result := ctrl.Result{}

	img := metal3.PreprovisioningImage{}
	err := r.Get(ctx, req.NamespacedName, &img)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			log.Info("PreprovisioningImage not found")
			err = nil
		}
		return result, err
	}

	changed, err := r.reconcile(ctx, &img)
	if k8serrors.IsNotFound(err) {
		delay := getErrorRetryDelay(img.Status)
		log.Info("requeuing to check for secret", "after", delay)
		result.RequeueAfter = delay
	}
	if changed {
		log.Info("updating status")
		err = r.Status().Update(ctx, &img)
	}

	return result, err
}

func (r *PreprovisioningImageReconciler) reconcile(ctx context.Context, img *metal3.PreprovisioningImage) (bool, error) {
	log := ctrl.LoggerFrom(ctx)
	generation := img.GetGeneration()

	secretManager := secretutils.NewSecretManager(log, r.Client, r.APIReader)
	secret, err := getNetworkDataSecret(secretManager, img)
	if k8serrors.IsNotFound(err) {
		return setError(ctx, generation, &img.Status, reasonMissingNetworkData, "NetworkData secret not found"), err
	}
	if err != nil {
		return setError(ctx, generation, &img.Status, reasonUnexpectedError, err.Error()), err
	}

	netData, err := gatherNetworkData(secret)
	if err != nil {
		return setError(ctx, generation, &img.Status, reasonConfigurationError, err.Error()), err
	}

	format := metal3.ImageFormatISO
	imageName := img.Name + ".qcow"

	url, err := r.ImageFileServer.ServeImage(imageName, netData)
	if err != nil {
		return setError(ctx, generation, &img.Status, reasonImageServingError, err.Error()), err
	}

	secretStatus := metal3.SecretStatus{}
	if secret != nil {
		secretStatus.Name = secret.Name
		secretStatus.Version = secret.GetResourceVersion()
	}

	log.Info("image available", "url", url, "format", format)
	return setImage(generation, &img.Status, url, format, secretStatus, img.Spec.Architecture, "Image available"), nil
}

func getErrorRetryDelay(status metal3.PreprovisioningImageStatus) time.Duration {
	errorCond := meta.FindStatusCondition(status.Conditions, string(metal3.ConditionImageError))
	if errorCond == nil || errorCond.Status != metav1.ConditionTrue {
		return 0
	}

	// exponential delay
	delay := time.Since(errorCond.LastTransitionTime.Time) + minRetryDelay

	if delay > maxRetryDelay {
		return maxRetryDelay
	}
	return delay
}

func gatherNetworkData(secret *corev1.Secret) ([]byte, error) {
	if secret == nil {
		return nil, nil
	}
	// TODO not yet sure what to do here, this is just something for testing.
	netData, ok := secret.Data["network"]
	if !ok {
		return nil, errors.New("network data in the secret has the incorrect format")
	}
	return netData, nil
}

func getNetworkDataSecret(secretManager secretutils.SecretManager, img *metal3.PreprovisioningImage) (*corev1.Secret, error) {
	networkDataSecret := img.Spec.NetworkDataName
	if networkDataSecret == "" {
		return nil, nil
	}

	secretKey := client.ObjectKey{
		Name:      networkDataSecret,
		Namespace: img.ObjectMeta.Namespace,
	}
	return secretManager.AcquireSecret(secretKey, img, false)
}

func setImage(generation int64, status *metal3.PreprovisioningImageStatus, url string,
	format metal3.ImageFormat, networkData metal3.SecretStatus, arch string,
	message string) bool {
	newStatus := status.DeepCopy()
	newStatus.ImageUrl = url
	newStatus.Format = format
	newStatus.Checksum = ""
	newStatus.ChecksumType = ""
	newStatus.Architecture = arch
	newStatus.NetworkData = networkData

	meta.SetStatusCondition(&status.Conditions, metav1.Condition{
		Type:               string(metal3.ConditionImageReady),
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		ObservedGeneration: generation,
		Reason:             string(reasonSuccess),
		Message:            message,
	})
	meta.SetStatusCondition(&status.Conditions, metav1.Condition{
		Type:               string(metal3.ConditionImageError),
		Status:             metav1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		ObservedGeneration: generation,
		Reason:             string(reasonSuccess),
		Message:            "",
	})

	changed := !apiequality.Semantic.DeepEqual(status, &newStatus)
	*status = *newStatus
	return changed
}

func setError(ctx context.Context, generation int64, status *metal3.PreprovisioningImageStatus, reason conditionReason, message string) bool {
	log := ctrl.LoggerFrom(ctx)

	newStatus := status.DeepCopy()
	newStatus.ImageUrl = ""
	newStatus.Checksum = ""
	newStatus.ChecksumType = ""

	log.Info("error condition", "reason", reason, "message", message)

	meta.SetStatusCondition(&status.Conditions, metav1.Condition{
		Type:               string(metal3.ConditionImageReady),
		Status:             metav1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		ObservedGeneration: generation,
		Reason:             string(reason),
		Message:            "",
	})
	meta.SetStatusCondition(&status.Conditions, metav1.Condition{
		Type:               string(metal3.ConditionImageError),
		Status:             metav1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		ObservedGeneration: generation,
		Reason:             string(reason),
		Message:            message,
	})

	changed := !apiequality.Semantic.DeepEqual(status, &newStatus)
	*status = *newStatus
	return changed
}

func (r *PreprovisioningImageReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&metal3.PreprovisioningImage{}).
		Owns(&corev1.Secret{}).
		Complete(r)
}
