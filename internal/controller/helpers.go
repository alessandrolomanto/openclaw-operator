package controller

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	openclawv1alpha1 "github.com/alessandrolomanto/openclaw-operator/api/v1alpha1"
)

func applyDefaults(oc *openclawv1alpha1.OpenClawInstance) {
	if oc.Spec.Image.Repository == "" {
		oc.Spec.Image.Repository = "ghcr.io/openclaw/openclaw"
	}
	if oc.Spec.Image.Tag == "" {
		oc.Spec.Image.Tag = "latest"
	}
	if oc.Spec.Storage.Size == "" {
		oc.Spec.Storage.Size = "10Gi"
	}
	if oc.Spec.Ollama != nil && oc.Spec.Ollama.Enabled {
		if oc.Spec.Ollama.Image.Repository == "" {
			oc.Spec.Ollama.Image.Repository = "ollama/ollama"
		}
		if oc.Spec.Ollama.Image.Tag == "" {
			oc.Spec.Ollama.Image.Tag = "latest"
		}
		if oc.Spec.Ollama.Storage.Size == "" {
			oc.Spec.Ollama.Storage.Size = "50Gi"
		}
	}
}

func computeConfigHash(oc *openclawv1alpha1.OpenClawInstance) string {
	data, _ := json.Marshal(oc.Spec.Config)
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h[:8])
}

func computeToolsHash(tools []string) string {
	h := sha256.Sum256([]byte(strings.Join(tools, ",")))
	return fmt.Sprintf("%x", h[:8])
}

func labelsForInstance(name string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "openclaw",
		"app.kubernetes.io/instance":   name,
		"app.kubernetes.io/managed-by": "openclaw-operator",
	}
}

func labelsForOllama(name string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "ollama",
		"app.kubernetes.io/instance":   name,
		"app.kubernetes.io/managed-by": "openclaw-operator",
	}
}

func (r *OpenClawInstanceReconciler) setPhase(
	ctx context.Context, oc *openclawv1alpha1.OpenClawInstance, phase string,
) error {
	oc.Status.Phase = phase
	return r.Status().Update(ctx, oc)
}

func (r *OpenClawInstanceReconciler) setCondition(
	oc *openclawv1alpha1.OpenClawInstance,
	condType string, ready bool, reason, message string,
) {
	status := metav1.ConditionFalse
	if ready {
		status = metav1.ConditionTrue
	}
	cond := metav1.Condition{
		Type:               condType,
		Status:             status,
		ObservedGeneration: oc.Generation,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
	for i, c := range oc.Status.Conditions {
		if c.Type == condType {
			oc.Status.Conditions[i] = cond
			return
		}
	}
	oc.Status.Conditions = append(oc.Status.Conditions, cond)
}

// determinePhase checks actual readiness of managed StatefulSets.
// Returns (phase, ready, message).
func (r *OpenClawInstanceReconciler) determinePhase(
	ctx context.Context, oc *openclawv1alpha1.OpenClawInstance,
) (string, bool, string) {
	// Check gateway StatefulSet
	gatewaySts := &appsv1.StatefulSet{}
	key := client.ObjectKey{Name: oc.Name, Namespace: oc.Namespace}
	if err := r.Get(ctx, key, gatewaySts); err != nil {
		return phaseProvisioning, false, "Gateway StatefulSet not found yet"
	}
	if gatewaySts.Status.ReadyReplicas < 1 {
		return phaseProvisioning, false, fmt.Sprintf(
			"Gateway: %d/%d replicas ready",
			gatewaySts.Status.ReadyReplicas, gatewaySts.Status.Replicas)
	}

	// Check Ollama StatefulSet if enabled
	if oc.Spec.Ollama != nil && oc.Spec.Ollama.Enabled {
		ollamaSts := &appsv1.StatefulSet{}
		ollamaKey := client.ObjectKey{Name: oc.Name + "-ollama", Namespace: oc.Namespace}
		if err := r.Get(ctx, ollamaKey, ollamaSts); err != nil {
			return phaseProvisioning, false, "Ollama StatefulSet not found yet"
		}
		if ollamaSts.Status.ReadyReplicas < 1 {
			return phaseProvisioning, false, fmt.Sprintf(
				"Ollama: %d/%d replicas ready",
				ollamaSts.Status.ReadyReplicas, ollamaSts.Status.Replicas)
		}
	}

	return phaseRunning, true, "All resources are ready"
}

func (r *OpenClawInstanceReconciler) handleDeletion(
	ctx context.Context, oc *openclawv1alpha1.OpenClawInstance,
) (ctrl.Result, error) {
	_ = r.setPhase(ctx, oc, phaseTerminating)
	controllerutil.RemoveFinalizer(oc, finalizerName)
	return ctrl.Result{}, r.Update(ctx, oc)
}

func (r *OpenClawInstanceReconciler) ensureFinalizer(
	ctx context.Context, oc *openclawv1alpha1.OpenClawInstance,
) (bool, error) {
	if controllerutil.ContainsFinalizer(oc, finalizerName) {
		return false, nil
	}
	controllerutil.AddFinalizer(oc, finalizerName)
	return true, r.Update(ctx, oc)
}
