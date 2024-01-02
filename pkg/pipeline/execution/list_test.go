package execution

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestGroupAndSortPipelineRuns tests the GroupAndSortPipelineRuns function.
func TestGroupAndSortPipelineRuns(t *testing.T) {
	tests := []struct {
		name     string
		runs     []PipelineRunValue
		expected map[string][]PipelineRunValue
	}{
		{
			name: "Group and sort pipeline runs",
			runs: []PipelineRunValue{
				{
					Name:              "run1",
					CreationTimestamp: metav1.Time{Time: time.Date(2024, 01, 02, 10, 00, 00, 00, time.UTC)},
					Namespace:         "ns1",
					CreatorPipeline:   "pipeline1",
				},
				{
					Name:              "run2",
					CreationTimestamp: metav1.Time{Time: time.Date(2024, 01, 02, 12, 00, 00, 00, time.UTC)},
					Namespace:         "ns2",
					CreatorPipeline:   "pipeline2",
				},
				{
					Name:              "run3",
					CreationTimestamp: metav1.Time{Time: time.Date(2024, 01, 02, 11, 00, 00, 00, time.UTC)},
					Namespace:         "ns1",
					CreatorPipeline:   "pipeline1",
				},
			},
			expected: map[string][]PipelineRunValue{
				"pipeline1": {
					{
						Name:              "run1",
						CreationTimestamp: metav1.Time{Time: time.Date(2024, 01, 02, 10, 00, 00, 00, time.UTC)},
						Namespace:         "ns1",
						CreatorPipeline:   "pipeline1",
					},
					{
						Name:              "run3",
						CreationTimestamp: metav1.Time{Time: time.Date(2024, 01, 02, 11, 00, 00, 00, time.UTC)},
						Namespace:         "ns1",
						CreatorPipeline:   "pipeline1",
					},
				},
				"pipeline2": {
					{
						Name:              "run2",
						CreationTimestamp: metav1.Time{Time: time.Date(2024, 01, 02, 12, 00, 00, 00, time.UTC)},
						Namespace:         "ns2",
						CreatorPipeline:   "pipeline2",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GroupAndSortPipelineRuns(tt.runs)
			assert.Equal(t, tt.expected, result)
		})
	}
}
