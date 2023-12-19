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
	"io/fs"
	pipelineapi "kurator.dev/kurator/pkg/apis/pipeline/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PredefinedTaskConfig struct {
	PipelineName      string
	PipelineNamespace string
	// TemplateName is set by user in `Pipeline.Tasks[i].PredefinedTask.Name`
	TemplateName string
	// Params is set by user in `Pipeline.Tasks[i].PredefinedTask.Params`
	Params         map[string]string
	OwnerReference *metav1.OwnerReference
}

// PredefinedTaskName is the name of Predefined task object, in case different pipeline have the same name task.
func (cfg PredefinedTaskConfig) PredefinedTaskName() string {
	return generatePipelineTaskName(cfg.TemplateName, cfg.PipelineName)
}

// RenderPredefinedTaskWithPipeline renders the full PredefinedTask configuration as a YAML byte array using **pipelineapi.CustomTask**.
func RenderPredefinedTaskWithPipeline(fsys fs.FS, taskName, pipelineName, pipelineNamespace string, task pipelineapi.PredefinedTask, ownerReference *metav1.OwnerReference) ([]byte, error) {
	cfg := PredefinedTaskConfig{
		PipelineName:      pipelineName,
		PipelineNamespace: pipelineNamespace,
		TemplateName:      string(task.Name),
		Params:            task.Params,
		OwnerReference:    ownerReference,
	}

	return renderTemplate(fsys, CustomTaskTemplateFile, CustomTaskTemplateName, cfg)
}

// RenderPredefinedTask renders the full PredefinedTask configuration as a YAML byte array using **PredefinedTaskConfig**.
func RenderPredefinedTask(fsys fs.FS, cfg PredefinedTaskConfig) ([]byte, error) {
	return renderTemplate(fsys, generateTaskTemplateFileName(cfg.TemplateName), generateTaskTemplateName(cfg.TemplateName), cfg)
}

func generateTaskTemplateFileName(Name string) string {
	return "predefined-task/" + Name + ".tpl"
}

func generateTaskTemplateName(taskType string) string {
	return "pipeline " + taskType + " task template"
}
