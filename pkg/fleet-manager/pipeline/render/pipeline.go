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
)

const (
	TektonPipelineNamePrefix = "tekton-"
)

type PipelineConfig struct {
	// PipelineName is the name of Pipeline. The Task will create at the same ns with the pipeline deployed
	PipelineName string
	// PipelineNamespace is the namespace of Pipeline. The Task will create at the same ns with the pipeline deployed
	PipelineNamespace string
	Tasks             []TaskTemplate
}

type TaskTemplate struct {
	// TaskType specifies the type of predefined task to be used.
	// This field is required to select the appropriate PredefinedTask.
	// +required
	TaskType string `json:"taskType"`

	// Params contains key-value pairs for task-specific parameters.
	// The required parameters vary depending on the TaskType chosen.
	// +optional
	Params map[string]string `json:"params,omitempty"`
}

// renderPipeline renders the Task configuration using a specified template.
func renderPipeline(fsys fs.FS, cfg TaskConfig) ([]byte, error) {
	return renderPipelineTemplate(fsys, generateTaskTemplateFileName(cfg.TaskType), generateTaskTemplateName(cfg.TaskType), cfg)
}

func (cfg PipelineConfig) TektonPipelineName() string {
	return TektonPipelineNamePrefix + cfg.PipelineName
}

//func generateTaskTemplateFileName(taskType string) string {
//	return taskType + ".tpl"
//}
//
//func generateTaskTemplateName(taskType string) string {
//	return "pipeline " + taskType + " template"
//}
