package v1alpha1

import (
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Pipeline is the top-level type for Kurator CI Pipeline.
type Pipeline struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PipelineSpec   `json:"spec"`
	Status PipelineStatus `json:"status,omitempty"`
}

// PipelineSpec defines the desired state of a Pipeline.
type PipelineSpec struct {
	// Description allows an administrator to provide a description of the pipeline.
	// +optional
	Description *string `json:"description,omitempty"`

	// Tasks is a list of tasks in the pipeline, containing detailed information about each task.
	Tasks []PipelineTask `json:"tasks"`

	// SharedWorkspace is the name of the PVC. If not specified, a PVC with the Pipeline's name as prefix will be created by default.
	// if not set, Kurator will create a pvc named Pipeline.name using default config
	// +optional
	SharedWorkspace *string `json:"sharedWorkspace,omitempty"`

	// Results are the list of results written out by the pipeline task's containers
	// +optional
	// +listType=atomic
	Results []tektonv1.PipelineRunResult `json:"results,omitempty"`
}

type PipelineTask struct {
	// Name is the name of the task.
	Name string `json:"name"`

	// PredefinedTask is a reference to a predefined task template.
	// Users should provide a PredefinedTask name from a predefined library.
	// +optional
	PredefinedTask *PredefinedTask `json:"predefinedTask,omitempty"`

	// CustomTask enables defining a task directly within the CRD if TaskRef is not used.
	// This should only be used when TaskRef is not provided.
	// +optional
	CustomTask *CustomTask `json:"customTask,omitempty"`

	// Retries represents how many times this task should be retried in case of task failure.
	// +optional
	Retries *int `json:"retries,omitempty"`
}

type PredefinedTask struct {
	// TaskType is used to specify the type of the predefined task.
	// This is a required field and determines which task template will be used.
	TaskType string `json:"taskType"`

	// Params are key-value pairs of parameters for the predefined task.
	// These parameters depend on the selected task template.
	// +optional
	Params map[string]string `json:"params,omitempty"`
}

// CustomTask defines the specification for a user-defined task.
type CustomTask struct {
	// Image specifies the Docker image name.
	// More info: https://kubernetes.io/docs/concepts/containers/images
	// +optional
	Image string `json:"image,omitempty" protobuf:"bytes,2,opt,name=image"`

	// Command is the entrypoint array. It's not executed in a shell.
	// If not provided, the image's ENTRYPOINT is used.
	// Environment variables can be used in the format $(VAR_NAME).
	// More info: https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell
	// +optional
	// +listType=atomic
	Command []string `json:"command,omitempty" protobuf:"bytes,3,rep,name=command"`

	// Args are the arguments for the entrypoint.
	// If not provided, the image's CMD is used.
	// Supports environment variable expansion in the format $(VAR_NAME).
	// More info: https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell
	// +optional
	// +listType=atomic
	Args []string `json:"args,omitempty" protobuf:"bytes,4,rep,name=args"`
	// Step's working directory.
	// If not specified, the container runtime's default will be used, which
	// might be configured in the container image.
	// Cannot be updated.
	// +optional
	WorkingDir string `json:"workingDir,omitempty" protobuf:"bytes,5,opt,name=workingDir"`
	// List of environment variables to set in the Step.
	// Cannot be updated.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	// +listType=atomic
	Env []corev1.EnvVar `json:"env,omitempty" patchStrategy:"merge" patchMergeKey:"name" protobuf:"bytes,7,rep,name=env"`
	// ResourceRequirements required by this Step.
	// Cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
	// +optional
	ResourceRequirements corev1.ResourceRequirements `json:"computeResources,omitempty" protobuf:"bytes,8,opt,name=computeResources"`
	// Script is the contents of an executable file to execute.
	//
	// If Script is not empty, the Step cannot have a Command and the Args will be passed to the Script.
	// +optional
	Script string `json:"script,omitempty"`
}

type PipelineStatus struct {
	// Phase describes the overall state of the Pipeline.
	// +optional
	Phase *string `json:"phase,omitempty"`
}
