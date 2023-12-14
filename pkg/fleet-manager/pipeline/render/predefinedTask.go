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

type PredefinedTaskConfig struct {
	PipelineName      string
	PipelineNamespace string
	// TaskName is set by user in `Pipeline.Tasks[i].Name`
	TaskName string
	// TemplateName is set by user in `Pipeline.Tasks[i].PredefinedTask.Name`
	TemplateName string
	// Params is set by user in `Pipeline.Tasks[i].PredefinedTask.Params`
	Params map[string]string
}

// PredefinedTaskName generates the Tekton Task resource name for the predefined task
func (cfg PredefinedTaskConfig) PredefinedTaskName() string {
	return cfg.TaskName + "-" + cfg.PipelineName
}

// RenderPredefinedTask renders the PredefinedTask configuration using a specified template.
func RenderPredefinedTask(fsys fs.FS, cfg PredefinedTaskConfig) ([]byte, error) {
	return renderTemplate(fsys, generateTaskTemplateFileName(cfg.TemplateName), generateTaskTemplateName(cfg.TemplateName), cfg)
}

func generateTaskTemplateFileName(Name string) string {
	return "predefinde-task/" + Name + ".tpl"
}

func generateTaskTemplateName(taskType string) string {
	return "pipeline " + taskType + " task template"
}
