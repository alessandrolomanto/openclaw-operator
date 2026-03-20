/*
Copyright 2026.

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

package controller

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	openclawv1alpha1 "github.com/alessandrolomanto/openclaw-operator/api/v1alpha1"
)

const (
	finalizerName  = "openclaw.nonnoalex.dev/finalizer"
	requeueAfter   = 5 * time.Minute
	configHashAnno = "openclaw.nonnoalex.dev/config-hash"
	toolsHashAnno  = "openclaw.nonnoalex.dev/tools-hash"
)

// +kubebuilder:rbac:groups=openclaw.nonnoalex.dev,resources=openclawinstances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=openclaw.nonnoalex.dev,resources=openclawinstances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=openclaw.nonnoalex.dev,resources=openclawinstances/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services;configmaps;persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

type OpenClawInstanceReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

func (r *OpenClawInstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// 1. Fetch the CR
	instance := &openclawv1alpha1.OpenClawInstance{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// 2. Handle deletion
	if !instance.DeletionTimestamp.IsZero() {
		r.Recorder.Event(instance, corev1.EventTypeNormal, "Deleting", "Cleaning up managed resources")
		return r.handleDeletion(ctx, instance)
	}

	// 3. Add finalizer if missing
	if added, err := r.ensureFinalizer(ctx, instance); err != nil || added {
		return ctrl.Result{Requeue: added}, err
	}

	// 4. Set initial phase
	if instance.Status.Phase == "" {
		r.Recorder.Event(instance, corev1.EventTypeNormal, "Provisioning", "Starting initial reconciliation")
		return ctrl.Result{Requeue: true}, r.setPhase(ctx, instance, "Pending")
	}
	if instance.Status.Phase == "Pending" {
		_ = r.setPhase(ctx, instance, "Provisioning")
	}

	// 5. Apply defaults for fields the user left empty
	applyDefaults(instance)

	// 6. Reconcile resources in dependency order
	steps := []struct {
		name string
		fn   func(context.Context, *openclawv1alpha1.OpenClawInstance) error
	}{
		{"ConfigMap", r.reconcileConfigMap},
		{"PVC", r.reconcilePVC},
		{"OllamaPVC", r.reconcileOllamaPVC},
		{"OllamaStatefulSet", r.reconcileOllamaStatefulSet},
		{"OllamaService", r.reconcileOllamaService},
		{"StatefulSet", r.reconcileStatefulSet},
		{"Service", r.reconcileService},
	}

	for _, step := range steps {
		if err := step.fn(ctx, instance); err != nil {
			log.Error(err, "reconciliation failed", "step", step.name)
			r.Recorder.Eventf(instance, corev1.EventTypeWarning, "ReconcileFailed",
				"Failed to reconcile %s: %v", step.name, err)
			_ = r.setPhase(ctx, instance, "Failed")
			r.setCondition(instance, "Ready", false,
				"ReconcileFailed", fmt.Sprintf("%s: %v", step.name, err))
			_ = r.Status().Update(ctx, instance)
			return ctrl.Result{RequeueAfter: time.Minute}, nil
		}
	}

	// 7. Check actual readiness of the StatefulSets
	phase, ready, message := r.determinePhase(ctx, instance)

	now := metav1.Now()
	instance.Status.Phase = phase
	instance.Status.ObservedGeneration = instance.Generation
	instance.Status.LastReconcileTime = &now
	instance.Status.GatewayEndpoint = fmt.Sprintf(
		"%s.%s.svc:18789", instance.Name, instance.Namespace)
	r.setCondition(instance, "Ready", ready, phase, message)
	if err := r.Status().Update(ctx, instance); err != nil {
		return ctrl.Result{}, err
	}

	if ready {
		r.Recorder.Event(instance, corev1.EventTypeNormal, "Reconciled", "All resources are ready")
	}

	log.Info("reconciliation complete", "phase", phase, "ready", ready)
	if !ready {
		return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
	}
	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

func (r *OpenClawInstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Recorder = mgr.GetEventRecorderFor("openclaw-operator")

	return ctrl.NewControllerManagedBy(mgr).
		For(&openclawv1alpha1.OpenClawInstance{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Named("openclawinstance").
		Complete(r)
}
