package controller

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	openclawv1alpha1 "github.com/alessandrolomanto/openclaw-operator/api/v1alpha1"
)

func (r *OpenClawInstanceReconciler) reconcileConfigMap(
	ctx context.Context, oc *openclawv1alpha1.OpenClawInstance,
) error {
	// If the user provides their own ConfigMap, skip creation
	if oc.Spec.Config != nil && oc.Spec.Config.ConfigMapRef != nil {
		return nil
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      oc.Name + "-config",
			Namespace: oc.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, cm, func() error {
		if err := ctrl.SetControllerReference(oc, cm, r.Scheme); err != nil {
			return err
		}

		configJSON := "{}"
		if oc.Spec.Config != nil && oc.Spec.Config.Raw != nil {
			configJSON = string(oc.Spec.Config.Raw.Raw)
		}

		cm.Data = map[string]string{
			"openclaw.json": configJSON,
		}
		return nil
	})
	return err
}
