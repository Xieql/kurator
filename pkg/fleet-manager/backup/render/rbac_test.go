package render

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"kurator.dev/kurator/pkg/fleet-manager/manifests"
)

var manifestFS = manifests.BuiltinOrDir("manifests")

const expectedRBACFilePath = "testdata/rbac/"

func TestRenderRBAC(t *testing.T) {
	// Define test cases including both valid and error scenarios.
	cases := []struct {
		name         string
		cfg          RBACConfig
		expectError  bool
		expectedFile string
	}{
		{
			name: "valid configuration",
			cfg: RBACConfig{
				PipelineName:      "example",
				PipelineNamespace: "default",
			},
			expectError:  false,
			expectedFile: "default-example.yaml",
		},
		{
			name: "empty PipelineName",
			cfg: RBACConfig{
				PipelineName:      "",
				PipelineNamespace: "default",
			},
			expectError: true,
		},
		{
			name: "empty PipelineNamespace",
			cfg: RBACConfig{
				PipelineName:      "example",
				PipelineNamespace: "",
			},
			expectError: true,
		},
		{
			name: "invalid file system path",
			cfg: RBACConfig{
				PipelineName:      "example",
				PipelineNamespace: "default",
			},
			expectError: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fs := manifestFS
			// Use an invalid file system for the relevant test case.
			if tc.name == "invalid file system path" {
				fs = manifests.BuiltinOrDir("invalid-path")
			}

			result, err := renderRBAC(fs, tc.cfg)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				expected, err := os.ReadFile(expectedRBACFilePath + tc.expectedFile)
				assert.NoError(t, err)
				assert.Equal(t, string(expected), string(result))
			}
		})
	}
}
