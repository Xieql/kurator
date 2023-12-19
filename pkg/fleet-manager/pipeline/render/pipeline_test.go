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
	pipelineapi "kurator.dev/kurator/pkg/apis/pipeline/v1alpha1"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRenderPipelineWithTasks(t *testing.T) {
	expectedPipelineFilePath := "testdata/pipeline/"
	pipelineName := "test-pipeline"
	pipelineNameSpace := "kurator-pipeline"
	cases := []struct {
		name         string
		tasks        []pipelineapi.PipelineTask
		expectError  bool
		expectedFile string
	}{
		{
			name: "valid pipeline configuration, contains tasks: git-clone, cat-readme, go-test",
			tasks: []pipelineapi.PipelineTask{
				{
					Name:           "git-clone",
					PredefinedTask: &pipelineapi.PredefinedTask{Name: GitCloneTask},
				},
				{
					Name:       "cat-readme",
					CustomTask: &pipelineapi.CustomTask{},
				},
				{
					Name:           "go-test",
					PredefinedTask: &pipelineapi.PredefinedTask{Name: GoTestTask},
				},
			},
			expectError:  false,
			expectedFile: "readme-test.yaml",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fs := manifestFS

			result, err := RenderPipelineWithTasks(fs, pipelineName, pipelineNameSpace, tc.tasks, nil)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				expected, err := os.ReadFile(expectedPipelineFilePath + tc.expectedFile)
				assert.NoError(t, err)
				assert.Equal(t, string(expected), string(result))
			}
		})
	}
}
