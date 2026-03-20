package controller

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	openclawv1alpha1 "github.com/alessandrolomanto/openclaw-operator/api/v1alpha1"
)

// reconcilePVC ensures the data PVC for the gateway exists.
// PVCs are immutable after creation — we only create, never update.
func (r *OpenClawInstanceReconciler) reconcilePVC(
	ctx context.Context, oc *openclawv1alpha1.OpenClawInstance,
) error {
	return r.ensurePVC(ctx, oc, oc.Name+"-data", oc.Spec.Storage.Size, oc.Spec.Storage.StorageClassName)
}

// reconcileOllamaPVC ensures the model-storage PVC for Ollama exists.
func (r *OpenClawInstanceReconciler) reconcileOllamaPVC(
	ctx context.Context, oc *openclawv1alpha1.OpenClawInstance,
) error {
	if oc.Spec.Ollama == nil || !oc.Spec.Ollama.Enabled {
		return nil
	}
	return r.ensurePVC(ctx, oc, oc.Name+"-ollama-models", oc.Spec.Ollama.Storage.Size, oc.Spec.Ollama.Storage.StorageClassName)
}

func (r *OpenClawInstanceReconciler) ensurePVC(
	ctx context.Context,
	oc *openclawv1alpha1.OpenClawInstance,
	name, size string,
	storageClass *string,
) error {
	pvc := &corev1.PersistentVolumeClaim{}
	key := client.ObjectKey{Name: name, Namespace: oc.Namespace}

	if err := r.Get(ctx, key, pvc); err == nil {
		return nil // already exists
	} else if !errors.IsNotFound(err) {
		return err
	}

	pvc = &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: oc.Namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(size),
				},
			},
			StorageClassName: storageClass,
		},
	}
	if err := ctrl.SetControllerReference(oc, pvc, r.Scheme); err != nil {
		return err
	}
	return r.Create(ctx, pvc)
}
