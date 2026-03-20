package controller

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	openclawv1alpha1 "github.com/alessandrolomanto/openclaw-operator/api/v1alpha1"
)

func (r *OpenClawInstanceReconciler) reconcileOllamaStatefulSet(
	ctx context.Context, oc *openclawv1alpha1.OpenClawInstance,
) error {
	if oc.Spec.Ollama == nil || !oc.Spec.Ollama.Enabled {
		return nil
	}

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      oc.Name + "-ollama",
			Namespace: oc.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, sts, func() error {
		if err := ctrl.SetControllerReference(oc, sts, r.Scheme); err != nil {
			return err
		}

		labels := labelsForOllama(oc.Name)
		image := fmt.Sprintf("%s:%s",
			oc.Spec.Ollama.Image.Repository, oc.Spec.Ollama.Image.Tag)

		sts.Spec = appsv1.StatefulSetSpec{
			Replicas:    ptr.To(int32(1)),
			ServiceName: oc.Name + "-ollama",
			Selector:    &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:    "ollama",
							Image:   image,
							Command: []string{"ollama", "serve"},
							Env: []corev1.EnvVar{
								{Name: "OLLAMA_HOST", Value: "0.0.0.0"},
							},
							Ports: []corev1.ContainerPort{
								{Name: "http", ContainerPort: 11434},
							},
							Resources: oc.Spec.Ollama.Resources,
							VolumeMounts: []corev1.VolumeMount{
								{Name: "models", MountPath: "/root/.ollama"},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt32(11434),
									},
								},
								InitialDelaySeconds: 15,
								PeriodSeconds:       10,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt32(11434),
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       5,
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "models",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: oc.Name + "-ollama-models",
								},
							},
						},
					},
				},
			},
		}
		return nil
	})
	return err
}

func (r *OpenClawInstanceReconciler) reconcileOllamaService(
	ctx context.Context, oc *openclawv1alpha1.OpenClawInstance,
) error {
	if oc.Spec.Ollama == nil || !oc.Spec.Ollama.Enabled {
		return nil
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      oc.Name + "-ollama",
			Namespace: oc.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, svc, func() error {
		if err := ctrl.SetControllerReference(oc, svc, r.Scheme); err != nil {
			return err
		}
		svc.Spec = corev1.ServiceSpec{
			Selector: labelsForOllama(oc.Name),
			Ports: []corev1.ServicePort{
				{Name: "http", Port: 11434, TargetPort: intstr.FromInt32(11434)},
			},
		}
		return nil
	})
	return err
}
