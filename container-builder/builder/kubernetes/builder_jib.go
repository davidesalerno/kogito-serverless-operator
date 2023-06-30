// Copyright 2023 Red Hat, Inc. and/or its affiliates
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
	"path"

	"github.com/kiegroup/kogito-serverless-operator/container-builder/api"
	"github.com/kiegroup/kogito-serverless-operator/container-builder/util"
	"github.com/kiegroup/kogito-serverless-operator/container-builder/util/log"
)

type jibScheduler struct {
	*scheduler
	JibTask *api.JibTask
}

type jibSchedulerHandler struct {
}

var _ schedulerHandler = &jibSchedulerHandler{}

const defaultContextDir = "/home/kogito/serverless-workflow-project/resources"

func (k jibSchedulerHandler) CreateScheduler(info ContainerBuilderInfo, buildCtx containerBuildContext) Scheduler {

	jibTask := api.JibTask{
		ContainerBuildBaseTask: api.ContainerBuildBaseTask{Name: "JibTask", PublishTask: api.PublishTask{
			//TODO: Make the ContextDir configurable via api
			ContextDir: path.Join(defaultContextDir),
			BaseImage:  info.Platform.Spec.BaseImage,
			Image:      info.FinalImageName,
			Registry:   info.Platform.Spec.Registry,
		}},
	}

	buildCtx.ContainerBuild = &api.ContainerBuild{
		Spec: api.ContainerBuildSpec{
			Tasks:    []api.ContainerBuildTask{{Jib: &jibTask}},
			Strategy: api.ContainerBuildStrategyPod,
			Timeout:  *info.Platform.Spec.Timeout,
		},
		Status: api.ContainerBuildStatus{},
	}
	buildCtx.ContainerBuild.Name = info.BuildUniqueName
	buildCtx.ContainerBuild.Namespace = info.Platform.Namespace

	sched := &jibScheduler{
		&scheduler{
			builder: builder{
				L:       log.WithName(util.ComponentName),
				Context: buildCtx,
			},
			Resources: make([]resource, 0),
		},
		&jibTask,
	}
	// we hold our own reference for the default methods to return the right object
	sched.Scheduler = sched
	return sched
}

func (k jibSchedulerHandler) CanHandle(info ContainerBuilderInfo) bool {
	return info.Platform.Spec.BuildStrategy == api.ContainerBuildStrategyPod && info.Platform.Spec.PublishStrategy == api.PlatformBuildPublishStrategyJib
}

func (sk *jibScheduler) Schedule() (*api.ContainerBuild, error) {
	// verify if we really need this
	for _, task := range sk.builder.Context.ContainerBuild.Spec.Tasks {
		if task.Jib != nil {
			task.Jib = sk.JibTask
			break
		}
	}
	return sk.scheduler.Schedule()
}
