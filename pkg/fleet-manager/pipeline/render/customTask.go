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

//type TaskConfig struct {
//	// PipelineNamespace is the namespace of Pipeline. The Task will create at the same ns with the pipeline deployed
//	PipelineNamespace string
//	// taskType is set by user in Pipeline.TaskRef.TaskType
//	TaskType string
//	// Params is set by user in Pipeline.TaskRef.Params
//	Params map[string]string
//}
//
//// renderTask renders the Task configuration using a specified template.
//func renderTask(fsys fs.FS, cfg TaskConfig) ([]byte, error) {
//	return renderPipelineTemplate(fsys, generateTaskTemplateFileName(cfg.TaskType), generateTaskTemplateName(cfg.TaskType), cfg)
//}
//
//func generateTaskTemplateFileName(taskType string) string {
//	return taskType + ".tpl"
//}
//
//func generateTaskTemplateName(taskType string) string {
//	return "pipeline " + taskType + " template"
//}
