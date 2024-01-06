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
	pipelineapi "kurator.dev/kurator/pkg/apis/pipeline/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// GitCloneTask is the predefined task template name of git clone task
	GitCloneTask = "git-clone"
	// GoTestTask is the predefined task template name of go test task
	GoTestTask = "go-test"
	// GoLintTask is the predefined task template name of golangci lint task
	GoLintTask = "go-lint"
	// BuildPushImage is the predefined task template name of task about building image and pushing it to image repo
	BuildPushImage = "build-and-push-image"
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
	return cfg.TemplateName + "-" + cfg.PipelineName
}

// RenderPredefinedTaskWithPipeline renders the full PredefinedTask configuration as a YAML byte array using pipeline and pipelineapi.CustomTask.
func RenderPredefinedTaskWithPipeline(pipeline *pipelineapi.Pipeline, task *pipelineapi.PredefinedTask) ([]byte, error) {
	cfg := PredefinedTaskConfig{
		PipelineName:      pipeline.Name,
		PipelineNamespace: pipeline.Namespace,
		TemplateName:      string(task.Name),
		Params:            task.Params,
		OwnerReference:    GeneratePipelineOwnerRef(pipeline),
	}

	return RenderPredefinedTask(cfg)
}

// RenderPredefinedTask renders the full PredefinedTask configuration as a YAML byte array using PredefinedTaskConfig.
func RenderPredefinedTask(cfg PredefinedTaskConfig) ([]byte, error) {
	templateContent, err := getPredefinedTaskTemplate(cfg.TemplateName)
	if err != nil {
		fmt.Errorf("faild to getPredefinedTaskTemplate '%v' ", err)
		return nil, err
	}
	return renderTemplate(templateContent, generateTaskTemplateName(cfg.TemplateName), cfg)
}

func generateTaskTemplateName(taskType string) string {
	return "pipeline " + taskType + " task template"
}

// getPredefinedTaskTemplate is a function that returns the template string based on the given template name.
// It takes a template name as a parameter and returns two values: the template string and an error object.
// If the corresponding template is found in the templates map, it returns the template string and nil (indicating no error).
// If the template is not found, it returns an empty string and a custom error message.
func getPredefinedTaskTemplate(name string) (string, error) {
	if template, ok := predefinedTaskTemplates[name]; ok {
		return template, nil
	}
	// Returns an error if the template is not found
	return "", fmt.Errorf("Template named '%s' not found", name)
}

var predefinedTaskTemplates = map[string]string{
	GitCloneTask:   GitCloneTaskContent,
	GoTestTask:     GoTestTaskContent,
	GoLintTask:     GoLintTaskContent,
	BuildPushImage: BuildPushImageContent,
}
