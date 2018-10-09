/*
 * Copyright 2018 SUSE LINUX GmbH, Nuernberg, Germany..
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package manager

import (
	"path/filepath"

	"github.com/golang/glog"
	"github.com/kubernetes/kubernetes/cmd/kubeadm/app/util/apiclient"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	kubeadmutil "k8s.io/kubernetes/cmd/kubeadm/app/util"

	kubiccfg "github.com/kubic-project/kubic-init/pkg/config"
)

const (
	// the name for the Kubic manager
	kubicManagerName = "kubic-manager"
)

var (
	// number of replicas of the Kubic manager
	kubicManagerReplicas = int32(3)
)

// InstallKubicManager installs the kubic-manager by creating a deployment
func InstallKubicManager(cli clientset.Interface, config *kubiccfg.KubicInitConfiguration) error {

	glog.V(3).Infof("[kubic] creating kubic-manager Deployment '%s', with %d replicas...",
		kubicManagerName, kubicManagerReplicas)

	deployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kubicManagerName,
			Namespace: metav1.NamespaceSystem,
			Labels: map[string]string{
				"app": kubicManagerName,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": kubicManagerName,
				},
			},
			Replicas: &kubicManagerReplicas,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": kubicManagerName,
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyAlways,
					Tolerations: []corev1.Toleration{
						{
							Key:      "node-role.kubernetes.io/master",
							Operator: corev1.TolerationOpExists,
							Effect:   corev1.TaintEffectNoSchedule,
						},
						{
							Key:      "CriticalAddonsOnly",
							Operator: corev1.TolerationOpExists,
						},
					},
					Containers: []corev1.Container{
						{
							Name:            kubicManagerName,
							Image:           config.Manager.Image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command: []string{
								kubiccfg.DefaultKubicInitExeInstallPath,
								"manager",
								"-v5",
								"--config=" + kubiccfg.DefaultKubicInitConfig,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "kubic-config",
									MountPath: filepath.Dir(kubiccfg.DefaultKubicInitConfig),
									ReadOnly:  true,
								},
							},
						},
					},
					Affinity: &corev1.Affinity{
						PodAntiAffinity: &corev1.PodAntiAffinity{
							PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
								{
									Weight: 1,
									PodAffinityTerm: corev1.PodAffinityTerm{
										TopologyKey: "kubernetes.io/hostname",
										LabelSelector: &metav1.LabelSelector{
											MatchLabels: map[string]string{
												"app": kubicManagerName,
											},
										},
									},
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "kubic-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: kubiccfg.DefaultKubicInitConfigmap,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// just for debugging: pretty-print the YAML
	if glog.V(8) {
		marshalled, err := kubeadmutil.MarshalToYamlForCodecs(&deployment, appsv1.SchemeGroupVersion, scheme.Codecs)
		if err != nil {
			glog.V(1).Infof("[kubic] ERROR: when generating YAML for the kubic-manager Deployment: %s", err)
			return err
		}
		glog.Infof("[kubic] final kubic-manager Deployment produced:\n%s", marshalled)
	}

	glog.V(3).Infof("[kubic] installing kubic-manager with Deployment '%s'", deployment.GetName())
	return apiclient.CreateOrUpdateDeployment(cli, &deployment)
}
