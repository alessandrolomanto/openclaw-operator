package controller

import (
	"context"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	openclawv1alpha1 "github.com/alessandrolomanto/openclaw-operator/api/v1alpha1"
)

const (
	toolsBinPath = "/usr/local/tools/bin"
	toolsLibPath = "/usr/local/tools/lib"
	defaultPath  = toolsBinPath + ":/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
)

func (r *OpenClawInstanceReconciler) reconcileStatefulSet(
	ctx context.Context, oc *openclawv1alpha1.OpenClawInstance,
) error {
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      oc.Name,
			Namespace: oc.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, sts, func() error {
		if err := ctrl.SetControllerReference(oc, sts, r.Scheme); err != nil {
			return err
		}

		labels := labelsForInstance(oc.Name)
		annotations := map[string]string{
			configHashAnno: computeConfigHash(oc),
			toolsHashAnno:  computeToolsHash(oc.Spec.Tools),
		}

		initContainers := r.buildInitContainers(oc)
		containers := []corev1.Container{r.buildGatewayContainer(oc)}
		if oc.Spec.CLI != nil && oc.Spec.CLI.Enabled {
			containers = append(containers, r.buildCLIContainer(oc))
		}

		sts.Spec = appsv1.StatefulSetSpec{
			Replicas:    ptr.To(int32(1)),
			ServiceName: oc.Name,
			Selector:    &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: annotations,
				},
				Spec: corev1.PodSpec{
					SecurityContext: &corev1.PodSecurityContext{
						RunAsUser:  ptr.To(int64(1000)),
						RunAsGroup: ptr.To(int64(1000)),
						FSGroup:    ptr.To(int64(1000)),
					},
					InitContainers: initContainers,
					Containers:     containers,
					Volumes:        r.buildGatewayVolumes(oc),
				},
			},
		}
		return nil
	})
	return err
}

// --- Init containers ---

func (r *OpenClawInstanceReconciler) buildInitContainers(
	oc *openclawv1alpha1.OpenClawInstance,
) []corev1.Container {
	image := fmt.Sprintf("%s:%s", oc.Spec.Image.Repository, oc.Spec.Image.Tag)
	var inits []corev1.Container

	// init-config: seed or merge openclaw.json from ConfigMap into PVC
	inits = append(inits, corev1.Container{
		Name:    "init-config",
		Image:   image,
		Command: []string{"sh", "-c", buildConfigInitScript(oc)},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "data", MountPath: "/home/node/.openclaw"},
			{Name: "config-source", MountPath: "/config-source", ReadOnly: true},
		},
	})

	// init-tools: apt-get install tools, copy binaries to shared volume
	if len(oc.Spec.Tools) > 0 {
		inits = append(inits, corev1.Container{
			Name:    "init-tools",
			Image:   image,
			Command: []string{"sh", "-c", buildToolsInitScript(oc.Spec.Tools)},
			SecurityContext: &corev1.SecurityContext{
				RunAsUser: ptr.To(int64(0)),
			},
			VolumeMounts: []corev1.VolumeMount{
				{Name: "tools", MountPath: "/tools"},
			},
		})
	}

	return inits
}

func buildConfigInitScript(oc *openclawv1alpha1.OpenClawInstance) string {
	mergeMode := "merge"
	if oc.Spec.Config != nil && oc.Spec.Config.MergeMode == "overwrite" {
		mergeMode = "overwrite"
	}

	if mergeMode == "overwrite" {
		return `cp /config-source/openclaw.json /home/node/.openclaw/openclaw.json
echo "Config overwritten from ConfigMap"`
	}

	return `TARGET=/home/node/.openclaw/openclaw.json
SOURCE=/config-source/openclaw.json
if [ ! -f "$TARGET" ]; then
  echo "First boot: seeding config from ConfigMap"
  cp "$SOURCE" "$TARGET"
else
  echo "Merge mode: deep-merging ConfigMap into existing config"
  node -e "
    const fs = require('fs');
    const existing = JSON.parse(fs.readFileSync('$TARGET', 'utf8'));
    const base = JSON.parse(fs.readFileSync('$SOURCE', 'utf8'));
    const merge = (a, b) => {
      const result = {...a};
      for (const [k, v] of Object.entries(b)) {
        if (v && typeof v === 'object' && !Array.isArray(v) && result[k] && typeof result[k] === 'object') {
          result[k] = merge(result[k], v);
        } else if (!(k in result)) {
          result[k] = v;
        }
      }
      return result;
    };
    fs.writeFileSync('$TARGET', JSON.stringify(merge(existing, base), null, 2));
  "
fi`
}

func buildToolsInitScript(tools []string) string {
	toolList := strings.Join(tools, " ")
	return fmt.Sprintf(`#!/bin/sh
set -e
TOOLS_DIR=/tools
MARKER="$TOOLS_DIR/.installed"
REQUESTED="%s"

if [ -f "$MARKER" ] && [ "$(cat $MARKER)" = "$REQUESTED" ]; then
  echo "Tools already installed, skipping"
  exit 0
fi

mkdir -p "$TOOLS_DIR/bin" "$TOOLS_DIR/lib"

echo "Installing tools: $REQUESTED"
apt-get update -qq
apt-get install -y --no-install-recommends $REQUESTED

for tool in $REQUESTED; do
  BIN=$(which "$tool" 2>/dev/null || true)
  if [ -n "$BIN" ] && [ -f "$BIN" ]; then
    cp "$BIN" "$TOOLS_DIR/bin/"
    echo "  copied $BIN"
    # Copy shared library dependencies
    ldd "$BIN" 2>/dev/null | grep "=> /" | awk '{print $3}' | while read lib; do
      if [ -f "$lib" ]; then
        cp -n "$lib" "$TOOLS_DIR/lib/"
        echo "    lib: $lib"
      fi
    done
  else
    echo "  warning: $tool not found in PATH after install"
  fi
done

echo "$REQUESTED" > "$MARKER"
echo "Done"
`, toolList)
}

// --- Main containers ---

func (r *OpenClawInstanceReconciler) buildGatewayContainer(
	oc *openclawv1alpha1.OpenClawInstance,
) corev1.Container {
	image := fmt.Sprintf("%s:%s", oc.Spec.Image.Repository, oc.Spec.Image.Tag)

	env := append([]corev1.EnvVar{
		{Name: "NODE_ENV", Value: "production"},
		{Name: "HOME", Value: "/home/node"},
	}, oc.Spec.Env...)

	if len(oc.Spec.Tools) > 0 {
		env = append(env,
			corev1.EnvVar{Name: "PATH", Value: defaultPath},
			corev1.EnvVar{Name: "LD_LIBRARY_PATH", Value: toolsLibPath},
		)
	}

	mounts := []corev1.VolumeMount{
		{Name: "data", MountPath: "/home/node/.openclaw"},
	}
	if len(oc.Spec.Tools) > 0 {
		mounts = append(mounts, corev1.VolumeMount{Name: "tools", MountPath: "/usr/local/tools"})
	}

	return corev1.Container{
		Name:    "openclaw",
		Image:   image,
		Command: []string{"node", "/app/dist/index.js", "gateway"},
		Ports: []corev1.ContainerPort{
			{Name: "gateway", ContainerPort: 18789},
			{Name: "bridge", ContainerPort: 18790},
		},
		Env:          env,
		EnvFrom:      oc.Spec.EnvFrom,
		Resources:    oc.Spec.Resources,
		VolumeMounts: mounts,
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt32(18789),
				},
			},
			InitialDelaySeconds: 30,
			PeriodSeconds:       10,
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt32(18789),
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       5,
		},
	}
}

func (r *OpenClawInstanceReconciler) buildCLIContainer(
	oc *openclawv1alpha1.OpenClawInstance,
) corev1.Container {
	image := fmt.Sprintf("%s:%s", oc.Spec.Image.Repository, oc.Spec.Image.Tag)

	env := []corev1.EnvVar{
		{Name: "HOME", Value: "/home/node"},
	}
	if len(oc.Spec.Tools) > 0 {
		env = append(env,
			corev1.EnvVar{Name: "PATH", Value: defaultPath},
			corev1.EnvVar{Name: "LD_LIBRARY_PATH", Value: toolsLibPath},
		)
	}

	mounts := []corev1.VolumeMount{
		{Name: "data", MountPath: "/home/node/.openclaw"},
	}
	if len(oc.Spec.Tools) > 0 {
		mounts = append(mounts, corev1.VolumeMount{Name: "tools", MountPath: "/usr/local/tools"})
	}

	return corev1.Container{
		Name:         "cli",
		Image:        image,
		Command:      []string{"sleep", "infinity"},
		Stdin:        true,
		TTY:          true,
		Env:          env,
		EnvFrom:      oc.Spec.EnvFrom,
		VolumeMounts: mounts,
	}
}

// --- Volumes ---

func (r *OpenClawInstanceReconciler) buildGatewayVolumes(
	oc *openclawv1alpha1.OpenClawInstance,
) []corev1.Volume {
	configMapName := oc.Name + "-config"
	configKey := "openclaw.json"
	if oc.Spec.Config != nil && oc.Spec.Config.ConfigMapRef != nil {
		configMapName = oc.Spec.Config.ConfigMapRef.Name
		if oc.Spec.Config.ConfigMapRef.Key != "" {
			configKey = oc.Spec.Config.ConfigMapRef.Key
		}
	}

	volumes := []corev1.Volume{
		{
			Name: "data",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: oc.Name + "-data",
				},
			},
		},
		{
			Name: "config-source",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: configMapName},
					Items: []corev1.KeyToPath{
						{Key: configKey, Path: "openclaw.json"},
					},
				},
			},
		},
	}

	if len(oc.Spec.Tools) > 0 {
		volumes = append(volumes, corev1.Volume{
			Name: "tools",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})
	}

	return volumes
}
