// Copyright 2022 Red Hat, Inc. and/or its affiliates
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kubernetes

import (
	"context"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/kiegroup/kogito-serverless-operator/container-builder/api"
	"github.com/kiegroup/kogito-serverless-operator/container-builder/client"
	"github.com/kiegroup/kogito-serverless-operator/container-builder/util"
	"github.com/kiegroup/kogito-serverless-operator/container-builder/util/defaults"
	"github.com/kiegroup/kogito-serverless-operator/container-builder/util/registry"
)

var (
	gcrJibRegistrySecret = registrySecret{
		fileName:    "kaniko-secret.json",
		mountPath:   "/secret",
		destination: "kaniko-secret.json",
		refEnv:      "GOOGLE_APPLICATION_CREDENTIALS",
	}
	plainDockerJibRegistrySecret = registrySecret{
		fileName:    "config.json",
		mountPath:   "/kaniko/.docker",
		destination: "config.json",
	}
	standardDockerJibRegistrySecret = registrySecret{
		fileName:    corev1.DockerConfigJsonKey,
		mountPath:   "/kaniko/.docker",
		destination: "config.json",
	}

	jibRegistrySecrets = []registrySecret{
		gcrKanikoRegistrySecret,
		plainDockerKanikoRegistrySecret,
		standardDockerKanikoRegistrySecret,
	}
)

const defaultBuildBashScript = "/home/kogito/launch/build-app.sh"

func addJibTaskToPod(ctx context.Context, c client.Client, build *api.ContainerBuild, task *api.JibTask, pod *corev1.Pod) error {
	// TODO: perform an actual registry lookup based on the environment
	if task.Registry.Address == "" {
		err := registry.LookupInternalRegistry(ctx, c, build, &task.ContainerBuildBaseTask)
		if err != nil {
			return err
		}
	}

	affinity := &corev1.Affinity{}
	env := make([]corev1.EnvVar, 0)
	volumes := make([]corev1.Volume, 0)
	volumeMounts := make([]corev1.VolumeMount, 0)

	addRegistryDetailsIntoEnv(task.Registry, task.Image, &env)

	// TODO: should be handled by a mount build context handler instead since we can have many possibilities
	if err := addResourcesToVolume(ctx, c, task.PublishTask, build, &volumes, &volumeMounts); err != nil {
		return err
	}

	env = append(env, proxyFromEnvironment()...)

	env = append(env, additionalVarsForJib()...)

	container := corev1.Container{
		Name:            strings.ToLower(task.Name),
		Image:           defaults.JibExecutorImage,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Env:             env,
		Command:         []string{"sh"},
		//TODO: Make the bash script to execute for the build configurable via api?
		Args:         []string{defaultBuildBashScript, task.ContextDir},
		Resources:    task.Resources,
		VolumeMounts: volumeMounts,
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: util.Pbool(false),
			Privileged:               util.Pbool(false),
			RunAsNonRoot:             util.Pbool(true),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{corev1.Capability("ALL")},
			},
		},
	}

	// We may want to handle possible conflicts
	pod.Spec.Affinity = affinity
	pod.Spec.Volumes = append(pod.Spec.Volumes, volumes...)
	pod.Spec.Containers = append(pod.Spec.Containers, container)

	return nil
}

func additionalVarsForJib() []corev1.EnvVar {
	var envVars []corev1.EnvVar

	envVars = append(envVars, corev1.EnvVar{
		Name:  "CONTAINER_BUILD",
		Value: "true",
	})

	envVars = append(envVars, corev1.EnvVar{
		Name:  "QUARKUS_CONTAINER_IMAGE_PUSH",
		Value: "true",
	})

	return envVars
}

func addRegistrySecretIntoEnv(secretName string, env *[]corev1.EnvVar) {
	*env = append(*env, corev1.EnvVar{
		Name: "QUARKUS_CONTAINER_IMAGE_USERNAME",
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: secretName,
				},
				Key: "username",
			},
		},
	})
	*env = append(*env, corev1.EnvVar{
		Name: "QUARKUS_CONTAINER_IMAGE_PASSWORD",
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: secretName,
				},
				Key: "password",
			},
		},
	})
}

func addRegistryDetailsIntoEnv(registry api.ContainerRegistrySpec, imageNameAndTag string, env *[]corev1.EnvVar) {
	*env = append(*env, corev1.EnvVar{
		Name:  "QUARKUS_CONTAINER_IMAGE_IMAGE",
		Value: registry.Address + "/" + imageNameAndTag})

	if registry.Insecure {
		*env = append(*env, corev1.EnvVar{
			Name:  "QUARKUS_CONTAINER_IMAGE_INSECURE",
			Value: "true"})
	}
	if registry.Secret != "" {
		addRegistrySecretIntoEnv(registry.Secret, env)
	}
}
