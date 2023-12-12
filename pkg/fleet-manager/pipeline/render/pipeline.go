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

	pipelineapi "kurator.dev/kurator/pkg/apis/pipeline/v1alpha1"
)

const (
	PipelineTemplateFile = "pipeline.tpl"
	PipelineTemplateName = "pipeline template"
)

type PipelineConfig struct {
	// PipelineName specifies the name of the pipeline. Tasks are created in the same namespace as the deployed pipeline.
	PipelineName string

	// PipelineNamespace defines the namespace of the pipeline. Tasks are created in this namespace.
	PipelineNamespace string

	// TasksInfo contains the necessary information to integrate tasks into the pipeline.
	TasksInfo string
}

// renderPipeline renders the full pipeline configuration as a YAML byte array using a specified template and **Pipeline.Tasks**.
func renderPipelineWithTasks(fsys fs.FS, pipelineName, pipelineNameSpace string, tasks []pipelineapi.PipelineTask) ([]byte, error) {
	tasksInfo, err := GenerateTasksInfo(pipelineName, tasks)
	if err != nil {
		return nil, err
	}

	cfg := PipelineConfig{
		PipelineName:      pipelineName,
		PipelineNamespace: pipelineNameSpace,
		TasksInfo:         tasksInfo,
	}

	return renderPipeline(fsys, cfg)
}

// renderPipeline renders the full pipeline configuration as a YAML byte array using a specified template and **PipelineConfig**.
func renderPipeline(fsys fs.FS, cfg PipelineConfig) ([]byte, error) {
	return renderTemplate(fsys, PipelineTemplateFile, PipelineTemplateName, cfg)
}

// GenerateTasksInfo constructs TasksInfo, detailing the integration of tasks into a given pipeline.
// 这个方法这样实现的原因在于 我们 要求第一个任务必须固定为 git clone。
func GenerateTasksInfo(pipelineName string, tasks []pipelineapi.PipelineTask) (string, error) {
	var tasksBuilder strings.Builder
	// lastTask record the current taskAfter task. git-clone always the first task, so it will be the lastTask for second task.
	lastTask := GitCloneTask
	for _, task := range tasks {
		// skip the first git-clone task.
		if task.Name == GitCloneTask {
			continue
		}
		var taskYaml string
		if (task.CustomTask == nil && task.PredefinedTask == nil) || (task.CustomTask != nil && task.PredefinedTask != nil) {
			return "", fmt.Errorf("only exactly one of 'PredefinedTask' or 'CustomTask' must be set in 'PipelineTask'")
		}
		taskYaml = generateTaskInfo(task.Name, generatePipelineTaskName(task.Name, pipelineName), lastTask, task.Retries)
		// add taskYaml to tasksBuilder
		fmt.Fprintf(&tasksBuilder, "  %s", taskYaml)
		// ensure task execution order as defined by the user
		lastTask = task.Name
	}
	return tasksBuilder.String(), nil
}

// generateTaskInfo formats a single task's information, including its dependencies and retries, for inclusion in a pipeline.
// - taskName the name of current pipeline task
// - taskRefer is the name of Tekton task which this pipeline task referred
// - lastTask is 当前任务的前一个任务，这个变量用来约束任务按照用户设定的顺序执行
// - retries is 用户设定的该任务失败重试次数
func generateTaskInfo(taskName, taskRefer, lastTask string, retries int) string {
	var taskBuilder strings.Builder

	// define task name and reference
	fmt.Fprintf(&taskBuilder, "  - name: %s\n      taskRef:\n        name: %s\n", taskName, taskRefer)

	// dpecify dependency on the preceding task
	fmt.Fprintf(&taskBuilder, "      runAfter: [\"%s\"]\n", lastTask)

	// add fixed workspace configuration
	taskBuilder.WriteString("      workspaces:\n        - name: source\n          workspace: kurator-pipeline-shared-data\n")

	// Include retry configuration if applicable
	if retries > 0 {
		fmt.Fprintf(&taskBuilder, "  retries: %d\n", retries)
	}

	return taskBuilder.String()
}
