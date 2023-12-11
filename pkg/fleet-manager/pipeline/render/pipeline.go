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
	"strings"
)

const (
	TektonPipelineNamePrefix = "tekton-"
	GitCloneTask             = "git-clone"
	PipelineTemplateFile     = "pipeline.tpl"
	PipelineTemplateName     = "pipeline template"
)

type PipelineConfig struct {
	// PipelineName is the name of Pipeline. The Task will create at the same ns with the pipeline deployed
	PipelineName string
	// PipelineNamespace is the namespace of Pipeline. The Task will create at the same ns with the pipeline deployed
	PipelineNamespace string
	Tasks             []PipelineTaskConfig
}

type PipelineTaskConfig struct {
	name     string
	taskinfo string
	retries  string
}

// TektonPipelineName constructs the complete Tekton pipeline name by prefixing the pipeline name.
func (cfg PipelineConfig) TektonPipelineName() string {
	return TektonPipelineNamePrefix + cfg.PipelineName
}

// renderPipeline renders the full pipeline configuration as a YAML byte array using a specified template and PipelineConfig.
func renderPipeline(fsys fs.FS, cfg PipelineConfig) ([]byte, error) {
	return renderTemplate(fsys, PipelineTemplateFile, PipelineTemplateName, cfg)
}

// GenerateTaskInfo creates the YAML configuration info string from the PipelineTask slice.
// This taskInfo string will be a part of PipelineConfig and will be rendered with the pipeline.tpl.
// If the task from PipelineTask is a PredefinedTask, it will call the generatePredefinedTaskYAML function,
// otherwise, it will call the generateCustomTaskYAML function.
func GenerateTaskInfo(tasks []PipelineTask) (string, error) {
	var tasksBuilder strings.Builder
	lastTask := GitCloneTask
	for _, task := range tasks {
		var taskYaml string
		if (task.CustomTask == nil && task.PredefinedTask == nil) || (task.CustomTask != nil && task.PredefinedTask != nil) {
			return "", fmt.Errorf("only exactly one of 'PredefinedTask' or 'CustomTask' is set in 'PipelineTask'")
		}
		if task.PredefinedTask != nil {
			taskYaml = generatePredefinedTaskYAML(task.Name, task.Name, lastTask, task.Retries)
		}
		taskYaml = generateCustomTaskYAML(task.CustomTask)
		// make sure all tasks are strictly executed in the order defined by the user
		fmt.Fprintf(&tasksBuilder, "  %s", taskYaml)

	}
	return tasksBuilder.String(), nil
}

// generatePredefinedTaskYAML constructs the YAML configuration for a single predefined task.
func generatePredefinedTaskYAML(taskName, taskRefer, lastTask string, retries int) string {
	var taskBuilder strings.Builder

	// Add the task name and reference
	fmt.Fprintf(&taskBuilder, "- name: %s\n  taskRef:\n    name: %s\n", taskName, taskRefer)

	// Add the previous task this one depends on
	fmt.Fprintf(&taskBuilder, "  runAfter: [\"%s\"]\n", lastTask)

	taskBuilder.WriteString("  workspaces:\n    - name: source\n      workspace: kurator-pipeline-shared-data\n")

	if retries > 0 {
		fmt.Fprintf(&taskBuilder, "  retries: %d\n", retries)
	}

	return taskBuilder.String()
}

// generateCustomTaskYAML construct the YAML configuration for a single custom task.
// TODO: Implement this function to handle custom tasks.
func generateCustomTaskYAML(CustomTask *CustomTask) string {
	var taskBuilder strings.Builder
	// TODO: implement it

	return taskBuilder.String()
}

type PipelineTask struct {
	// Name is the name of the task.
	Name string `json:"name"`

	// PredefinedTask allows users to select a predefined task.
	// Users can choose a predefined from a set list and fill in their own parameters.
	// +optional
	PredefinedTask *PredefinedTask `json:"predefinedTask,omitempty"`

	// CustomTask enables defining a task directly within the CRD if TaskRef is not used.
	// This should only be used when TaskRef is not provided.
	// +optional
	CustomTask *CustomTask `json:"customTask,omitempty"`

	// Retries represents how many times this task should be retried in case of task failure.
	// default values is zero.
	// +optional
	Retries int `json:"retries,omitempty"`
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

type PredefinedTask struct{}

type CustomTask struct{}
