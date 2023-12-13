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
	TriggerTemplateFile = "trigger.tpl"
	TriggerTemplateName = "pipeline trigger template"
)

type TriggerConfig struct {
	PipelineName      string
	PipelineNamespace string
}

// ServiceAccountName is the service account used by trigger
func (cfg TriggerConfig) ServiceAccountName() string {
	return cfg.PipelineName
}

func RenderTrigger(fsys fs.FS, cfg TriggerConfig) ([]byte, error) {
	return renderTemplate(fsys, TriggerTemplateFile, TriggerTemplateName, cfg)
}
