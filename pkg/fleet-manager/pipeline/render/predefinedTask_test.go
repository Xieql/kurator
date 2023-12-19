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
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRenderPredefinedTask(t *testing.T) {
	expectedTaskFilePath := "testdata/predefined-task/"
	// Define test cases for various task templates and configurations.
	cases := []struct {
		name         string
		cfg          PredefinedTaskConfig
		expectError  bool
		expectedFile string
	}{
		// ---- Case: Default Configuration for Git Clone ----
		// This case tests the basic configuration of the 'git-clone' template.
		// It will not include auth, because auth will add in pipeline.
		{
			name: "git-clone with basic parameters",
			cfg: PredefinedTaskConfig{
				PipelineName:      "test-pipeline",
				PipelineNamespace: "kurator-pipeline",
				TemplateName:      "git-clone",
				Params:            map[string]string{},
			},
			expectError:  false,
			expectedFile: "git-clone.yaml",
		},

		// ---- Case: Default Configuration for Go Test ----
		// This case tests the default configuration of the 'go-test' template.
		// It uses the default namespace and relies on all default parameter values.
		{
			name: "go-test with default parameters",
			cfg: PredefinedTaskConfig{
				PipelineName:      "test-pipeline",
				PipelineNamespace: "default",
				TemplateName:      GoTestTask,
				Params:            map[string]string{},
			},
			expectError:  false,
			expectedFile: "go-test-default.yaml",
		},

		// ---- Case: Custom Configuration for Go Test ----
		// This case customizes the 'go-test' template: setting Go version to 1.20,
		// targeting the './pkg/...' package path, and specifying the Linux ARM architecture.
		{
			name: "go-test with custom parameters - Go 1.20, ./pkg/..., Linux ARM",
			cfg: PredefinedTaskConfig{
				PipelineName:      "test-pipeline",
				PipelineNamespace: "kurator-pipeline",
				TemplateName:      GoTestTask,
				Params: map[string]string{
					"packages": "./pkg/...",
					"version":  "1.20",
					"GOOS":     "linux",
					"GOARCH":   "arm",
				},
			},
			expectError:  false,
			expectedFile: "go-test-custom-value.yaml",
		},

		// TODO: Add more test cases here for different task templates or configurations...
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fs := manifestFS

			result, err := RenderPredefinedTask(fs, tc.cfg)

			// Test assertions
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				expected, err := os.ReadFile(expectedTaskFilePath + tc.expectedFile)
				assert.NoError(t, err)
				assert.Equal(t, string(expected), string(result))
			}
		})
	}
}
