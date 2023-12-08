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
)

type PipelineConfig struct {
	// PipelineName is the name of Pipeline. The Task will create at the same ns with the pipeline deployed
	PipelineName string
	// PipelineNamespace is the namespace of Pipeline. The Task will create at the same ns with the pipeline deployed
	PipelineNamespace string
	TasksInfo         []string
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

func GetPredefinedTaskInfo(task PredefinedTask, pipelineName string) string {

}

func GetCustomTaskInfo(task CustomTask) string {

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

type TaskInfo struct {
	TaskName  string
	TaskRefer string
	LastTask  string
	Retries   int
}

func GenerateTaskInfo(tasks []PipelineTask) string {
	lastTask := GitCloneTask
	for _, task := range tasks {

	}
}

func GenerateTaskYAML(taskName, taskRefer, lastTask string, retries int) string {

	var taskBuilder strings.Builder

	fmt.Fprintf(&taskBuilder, "- name: %s\n  taskRef:\n    name: %s\n", taskName, taskRefer)

	fmt.Fprintf(&taskBuilder, "  runAfter: [\"%s\"]\n", lastTask)

	taskBuilder.WriteString("  workspaces:\n    - name: source\n      workspace: kurator-pipeline-shared-data\n")
	if retries > 0 { // 检查 Retries 是否被显式设置
		fmt.Fprintf(&taskBuilder, "  retries: %d\n", retries)
	}

	return taskBuilder.String()
}
