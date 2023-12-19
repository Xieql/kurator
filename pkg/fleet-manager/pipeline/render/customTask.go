/*
Copyright Kurator Authors.

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

package render

import (
	"fmt"
	"io/fs"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pipelineapi "kurator.dev/kurator/pkg/apis/pipeline/v1alpha1"
)

const (
	CustomTaskTemplateFile = "custom-task/custom-task.tpl"
	CustomTaskTemplateName = "pipeline custom task template"
)

type CustomTaskConfig struct {
	CustomTaskName       string
	PipelineName         string
	PipelineNamespace    string
	Image                string
	Command              []string
	Args                 []string
	Env                  []corev1.EnvVar
	ResourceRequirements *corev1.ResourceRequirements
	Script               string
	OwnerReference       *metav1.OwnerReference
}

// RenderCustomTaskWithPipeline renders the full CustomTask configuration as a YAML byte array using **pipelineapi.CustomTask**.
func RenderCustomTaskWithPipeline(fsys fs.FS, taskName, pipelineName, pipelineNamespace string, task pipelineapi.CustomTask, ownerReference *metav1.OwnerReference) ([]byte, error) {
	cfg := CustomTaskConfig{
		// in case different pipeline have the same name task.
		CustomTaskName:       generatePipelineTaskName(taskName, pipelineName),
		PipelineName:         pipelineName,
		PipelineNamespace:    pipelineNamespace,
		Image:                task.Image,
		Command:              task.Command,
		Args:                 task.Args,
		Env:                  task.Env,
		ResourceRequirements: task.ResourceRequirements,
		Script:               task.Script,
		OwnerReference:       ownerReference,
	}

	return renderTemplate(fsys, CustomTaskTemplateFile, CustomTaskTemplateName, cfg)
}

// RenderCustomTaskWithConfig renders the full CustomTask configuration as a YAML byte array using **CustomTaskConfig**.
func RenderCustomTaskWithConfig(fsys fs.FS, cfg CustomTaskConfig) ([]byte, error) {
	if cfg.Image == "" || cfg.CustomTaskName == "" {
		return nil, fmt.Errorf("invalid RBACConfig: PipelineName and PipelineNamespace must not be empty")
	}
	return renderTemplate(fsys, CustomTaskTemplateFile, CustomTaskTemplateName, cfg)
}
