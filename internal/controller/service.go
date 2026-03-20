package controller

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	openclawv1alpha1 "github.com/alessandrolomanto/openclaw-operator/api/v1alpha1"
)

func (r *OpenClawInstanceReconciler) reconcileService(
	ctx context.Context, oc *openclawv1alpha1.OpenClawInstance,
) error {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      oc.Name,
			Namespace: oc.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, svc, func() error {
		if err := ctrl.SetControllerReference(oc, svc, r.Scheme); err != nil {
			return err
		}
		svc.Spec = corev1.ServiceSpec{
			Selector: labelsForInstance(oc.Name),
			Ports: []corev1.ServicePort{
				{Name: "gateway", Port: 18789, TargetPort: intstr.FromInt32(18789)},
				{Name: "bridge", Port: 18790, TargetPort: intstr.FromInt32(18790)},
			},
		}
		return nil
	})
	return err
}
