//go:build !ignore_autogenerated
// +build !ignore_autogenerated

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

// Code generated by deepcopy-gen. DO NOT EDIT.

package v1alpha1

import (
	v1beta1 "github.com/fluxcd/flagger/pkg/apis/flagger/v1beta1"
	v1alpha3 "github.com/fluxcd/flagger/pkg/apis/istio/v1alpha3"
	v2beta1 "github.com/fluxcd/helm-controller/api/v2beta1"
	apiv1beta2 "github.com/fluxcd/kustomize-controller/api/v1beta2"
	kustomize "github.com/fluxcd/pkg/apis/kustomize"
	meta "github.com/fluxcd/pkg/apis/meta"
	v1beta2 "github.com/fluxcd/source-controller/api/v1beta2"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Application) DeepCopyInto(out *Application) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Application.
func (in *Application) DeepCopy() *Application {
	if in == nil {
		return nil
	}
	out := new(Application)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Application) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ApplicationDestination) DeepCopyInto(out *ApplicationDestination) {
	*out = *in
	if in.ClusterSelector != nil {
		in, out := &in.ClusterSelector, &out.ClusterSelector
		*out = new(ClusterSelector)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ApplicationDestination.
func (in *ApplicationDestination) DeepCopy() *ApplicationDestination {
	if in == nil {
		return nil
	}
	out := new(ApplicationDestination)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ApplicationList) DeepCopyInto(out *ApplicationList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Application, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ApplicationList.
func (in *ApplicationList) DeepCopy() *ApplicationList {
	if in == nil {
		return nil
	}
	out := new(ApplicationList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ApplicationList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ApplicationSource) DeepCopyInto(out *ApplicationSource) {
	*out = *in
	if in.GitRepository != nil {
		in, out := &in.GitRepository, &out.GitRepository
		*out = new(v1beta2.GitRepositorySpec)
		(*in).DeepCopyInto(*out)
	}
	if in.HelmRepository != nil {
		in, out := &in.HelmRepository, &out.HelmRepository
		*out = new(v1beta2.HelmRepositorySpec)
		(*in).DeepCopyInto(*out)
	}
	if in.OCIRepository != nil {
		in, out := &in.OCIRepository, &out.OCIRepository
		*out = new(v1beta2.OCIRepositorySpec)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ApplicationSource.
func (in *ApplicationSource) DeepCopy() *ApplicationSource {
	if in == nil {
		return nil
	}
	out := new(ApplicationSource)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ApplicationSourceStatus) DeepCopyInto(out *ApplicationSourceStatus) {
	*out = *in
	if in.GitRepoStatus != nil {
		in, out := &in.GitRepoStatus, &out.GitRepoStatus
		*out = new(v1beta2.GitRepositoryStatus)
		(*in).DeepCopyInto(*out)
	}
	if in.HelmRepoStatus != nil {
		in, out := &in.HelmRepoStatus, &out.HelmRepoStatus
		*out = new(v1beta2.HelmRepositoryStatus)
		(*in).DeepCopyInto(*out)
	}
	if in.OCIRepoStatus != nil {
		in, out := &in.OCIRepoStatus, &out.OCIRepoStatus
		*out = new(v1beta2.OCIRepositoryStatus)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ApplicationSourceStatus.
func (in *ApplicationSourceStatus) DeepCopy() *ApplicationSourceStatus {
	if in == nil {
		return nil
	}
	out := new(ApplicationSourceStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ApplicationSpec) DeepCopyInto(out *ApplicationSpec) {
	*out = *in
	in.Source.DeepCopyInto(&out.Source)
	if in.SyncPolicies != nil {
		in, out := &in.SyncPolicies, &out.SyncPolicies
		*out = make([]*ApplicationSyncPolicy, len(*in))
		for i := range *in {
			if (*in)[i] != nil {
				in, out := &(*in)[i], &(*out)[i]
				*out = new(ApplicationSyncPolicy)
				(*in).DeepCopyInto(*out)
			}
		}
	}
	if in.Destination != nil {
		in, out := &in.Destination, &out.Destination
		*out = new(ApplicationDestination)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ApplicationSpec.
func (in *ApplicationSpec) DeepCopy() *ApplicationSpec {
	if in == nil {
		return nil
	}
	out := new(ApplicationSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ApplicationStatus) DeepCopyInto(out *ApplicationStatus) {
	*out = *in
	if in.SourceStatus != nil {
		in, out := &in.SourceStatus, &out.SourceStatus
		*out = new(ApplicationSourceStatus)
		(*in).DeepCopyInto(*out)
	}
	if in.SyncStatus != nil {
		in, out := &in.SyncStatus, &out.SyncStatus
		*out = make([]*ApplicationSyncStatus, len(*in))
		for i := range *in {
			if (*in)[i] != nil {
				in, out := &(*in)[i], &(*out)[i]
				*out = new(ApplicationSyncStatus)
				(*in).DeepCopyInto(*out)
			}
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ApplicationStatus.
func (in *ApplicationStatus) DeepCopy() *ApplicationStatus {
	if in == nil {
		return nil
	}
	out := new(ApplicationStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ApplicationSyncPolicy) DeepCopyInto(out *ApplicationSyncPolicy) {
	*out = *in
	if in.Kustomization != nil {
		in, out := &in.Kustomization, &out.Kustomization
		*out = new(Kustomization)
		(*in).DeepCopyInto(*out)
	}
	if in.Helm != nil {
		in, out := &in.Helm, &out.Helm
		*out = new(HelmRelease)
		(*in).DeepCopyInto(*out)
	}
	if in.Destination != nil {
		in, out := &in.Destination, &out.Destination
		*out = new(ApplicationDestination)
		(*in).DeepCopyInto(*out)
	}
	if in.Rollout != nil {
		in, out := &in.Rollout, &out.Rollout
		*out = new(RolloutConfig)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ApplicationSyncPolicy.
func (in *ApplicationSyncPolicy) DeepCopy() *ApplicationSyncPolicy {
	if in == nil {
		return nil
	}
	out := new(ApplicationSyncPolicy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ApplicationSyncStatus) DeepCopyInto(out *ApplicationSyncStatus) {
	*out = *in
	if in.KustomizationStatus != nil {
		in, out := &in.KustomizationStatus, &out.KustomizationStatus
		*out = new(apiv1beta2.KustomizationStatus)
		(*in).DeepCopyInto(*out)
	}
	if in.HelmReleaseStatus != nil {
		in, out := &in.HelmReleaseStatus, &out.HelmReleaseStatus
		*out = new(v2beta1.HelmReleaseStatus)
		(*in).DeepCopyInto(*out)
	}
	if in.RolloutStatus != nil {
		in, out := &in.RolloutStatus, &out.RolloutStatus
		*out = new(RolloutStatus)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ApplicationSyncStatus.
func (in *ApplicationSyncStatus) DeepCopy() *ApplicationSyncStatus {
	if in == nil {
		return nil
	}
	out := new(ApplicationSyncStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CanaryConfig) DeepCopyInto(out *CanaryConfig) {
	*out = *in
	if in.StepWeights != nil {
		in, out := &in.StepWeights, &out.StepWeights
		*out = make([]int, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CanaryConfig.
func (in *CanaryConfig) DeepCopy() *CanaryConfig {
	if in == nil {
		return nil
	}
	out := new(CanaryConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CanaryThresholdRange) DeepCopyInto(out *CanaryThresholdRange) {
	*out = *in
	if in.Min != nil {
		in, out := &in.Min, &out.Min
		*out = new(float64)
		**out = **in
	}
	if in.Max != nil {
		in, out := &in.Max, &out.Max
		*out = new(float64)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CanaryThresholdRange.
func (in *CanaryThresholdRange) DeepCopy() *CanaryThresholdRange {
	if in == nil {
		return nil
	}
	out := new(CanaryThresholdRange)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterSelector) DeepCopyInto(out *ClusterSelector) {
	*out = *in
	if in.MatchLabels != nil {
		in, out := &in.MatchLabels, &out.MatchLabels
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterSelector.
func (in *ClusterSelector) DeepCopy() *ClusterSelector {
	if in == nil {
		return nil
	}
	out := new(ClusterSelector)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CommonMetadata) DeepCopyInto(out *CommonMetadata) {
	*out = *in
	if in.Annotations != nil {
		in, out := &in.Annotations, &out.Annotations
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Labels != nil {
		in, out := &in.Labels, &out.Labels
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CommonMetadata.
func (in *CommonMetadata) DeepCopy() *CommonMetadata {
	if in == nil {
		return nil
	}
	out := new(CommonMetadata)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CrossNamespaceObjectReference) DeepCopyInto(out *CrossNamespaceObjectReference) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CrossNamespaceObjectReference.
func (in *CrossNamespaceObjectReference) DeepCopy() *CrossNamespaceObjectReference {
	if in == nil {
		return nil
	}
	out := new(CrossNamespaceObjectReference)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CustomMetadata) DeepCopyInto(out *CustomMetadata) {
	*out = *in
	if in.Labels != nil {
		in, out := &in.Labels, &out.Labels
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Annotations != nil {
		in, out := &in.Annotations, &out.Annotations
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CustomMetadata.
func (in *CustomMetadata) DeepCopy() *CustomMetadata {
	if in == nil {
		return nil
	}
	out := new(CustomMetadata)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HelmChartTemplate) DeepCopyInto(out *HelmChartTemplate) {
	*out = *in
	if in.ObjectMeta != nil {
		in, out := &in.ObjectMeta, &out.ObjectMeta
		*out = new(HelmChartTemplateObjectMeta)
		(*in).DeepCopyInto(*out)
	}
	in.Spec.DeepCopyInto(&out.Spec)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HelmChartTemplate.
func (in *HelmChartTemplate) DeepCopy() *HelmChartTemplate {
	if in == nil {
		return nil
	}
	out := new(HelmChartTemplate)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HelmChartTemplateObjectMeta) DeepCopyInto(out *HelmChartTemplateObjectMeta) {
	*out = *in
	if in.Labels != nil {
		in, out := &in.Labels, &out.Labels
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Annotations != nil {
		in, out := &in.Annotations, &out.Annotations
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HelmChartTemplateObjectMeta.
func (in *HelmChartTemplateObjectMeta) DeepCopy() *HelmChartTemplateObjectMeta {
	if in == nil {
		return nil
	}
	out := new(HelmChartTemplateObjectMeta)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HelmChartTemplateSpec) DeepCopyInto(out *HelmChartTemplateSpec) {
	*out = *in
	if in.Interval != nil {
		in, out := &in.Interval, &out.Interval
		*out = new(v1.Duration)
		**out = **in
	}
	if in.ValuesFiles != nil {
		in, out := &in.ValuesFiles, &out.ValuesFiles
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HelmChartTemplateSpec.
func (in *HelmChartTemplateSpec) DeepCopy() *HelmChartTemplateSpec {
	if in == nil {
		return nil
	}
	out := new(HelmChartTemplateSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HelmRelease) DeepCopyInto(out *HelmRelease) {
	*out = *in
	in.Chart.DeepCopyInto(&out.Chart)
	out.Interval = in.Interval
	if in.DependsOn != nil {
		in, out := &in.DependsOn, &out.DependsOn
		*out = make([]meta.NamespacedObjectReference, len(*in))
		copy(*out, *in)
	}
	if in.Timeout != nil {
		in, out := &in.Timeout, &out.Timeout
		*out = new(v1.Duration)
		**out = **in
	}
	if in.MaxHistory != nil {
		in, out := &in.MaxHistory, &out.MaxHistory
		*out = new(int)
		**out = **in
	}
	if in.PersistentClient != nil {
		in, out := &in.PersistentClient, &out.PersistentClient
		*out = new(bool)
		**out = **in
	}
	if in.Install != nil {
		in, out := &in.Install, &out.Install
		*out = new(v2beta1.Install)
		(*in).DeepCopyInto(*out)
	}
	if in.Upgrade != nil {
		in, out := &in.Upgrade, &out.Upgrade
		*out = new(v2beta1.Upgrade)
		(*in).DeepCopyInto(*out)
	}
	if in.Rollback != nil {
		in, out := &in.Rollback, &out.Rollback
		*out = new(v2beta1.Rollback)
		(*in).DeepCopyInto(*out)
	}
	if in.Uninstall != nil {
		in, out := &in.Uninstall, &out.Uninstall
		*out = new(v2beta1.Uninstall)
		(*in).DeepCopyInto(*out)
	}
	if in.ValuesFrom != nil {
		in, out := &in.ValuesFrom, &out.ValuesFrom
		*out = make([]v2beta1.ValuesReference, len(*in))
		copy(*out, *in)
	}
	if in.Values != nil {
		in, out := &in.Values, &out.Values
		*out = new(apiextensionsv1.JSON)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HelmRelease.
func (in *HelmRelease) DeepCopy() *HelmRelease {
	if in == nil {
		return nil
	}
	out := new(HelmRelease)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Kustomization) DeepCopyInto(out *Kustomization) {
	*out = *in
	if in.CommonMetadata != nil {
		in, out := &in.CommonMetadata, &out.CommonMetadata
		*out = new(CommonMetadata)
		(*in).DeepCopyInto(*out)
	}
	if in.DependsOn != nil {
		in, out := &in.DependsOn, &out.DependsOn
		*out = make([]meta.NamespacedObjectReference, len(*in))
		copy(*out, *in)
	}
	out.Interval = in.Interval
	if in.RetryInterval != nil {
		in, out := &in.RetryInterval, &out.RetryInterval
		*out = new(v1.Duration)
		**out = **in
	}
	if in.Patches != nil {
		in, out := &in.Patches, &out.Patches
		*out = make([]kustomize.Patch, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Images != nil {
		in, out := &in.Images, &out.Images
		*out = make([]kustomize.Image, len(*in))
		copy(*out, *in)
	}
	if in.Timeout != nil {
		in, out := &in.Timeout, &out.Timeout
		*out = new(v1.Duration)
		**out = **in
	}
	if in.Components != nil {
		in, out := &in.Components, &out.Components
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Kustomization.
func (in *Kustomization) DeepCopy() *Kustomization {
	if in == nil {
		return nil
	}
	out := new(Kustomization)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Metric) DeepCopyInto(out *Metric) {
	*out = *in
	if in.IntervalSeconds != nil {
		in, out := &in.IntervalSeconds, &out.IntervalSeconds
		*out = new(int)
		**out = **in
	}
	if in.ThresholdRange != nil {
		in, out := &in.ThresholdRange, &out.ThresholdRange
		*out = new(CanaryThresholdRange)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Metric.
func (in *Metric) DeepCopy() *Metric {
	if in == nil {
		return nil
	}
	out := new(Metric)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RolloutConfig) DeepCopyInto(out *RolloutConfig) {
	*out = *in
	if in.TestLoader != nil {
		in, out := &in.TestLoader, &out.TestLoader
		*out = new(bool)
		**out = **in
	}
	if in.Workload != nil {
		in, out := &in.Workload, &out.Workload
		*out = new(CrossNamespaceObjectReference)
		**out = **in
	}
	if in.Primary != nil {
		in, out := &in.Primary, &out.Primary
		*out = new(CustomMetadata)
		(*in).DeepCopyInto(*out)
	}
	if in.Preview != nil {
		in, out := &in.Preview, &out.Preview
		*out = new(CustomMetadata)
		(*in).DeepCopyInto(*out)
	}
	if in.RolloutPolicy != nil {
		in, out := &in.RolloutPolicy, &out.RolloutPolicy
		*out = new(RolloutPolicy)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RolloutConfig.
func (in *RolloutConfig) DeepCopy() *RolloutConfig {
	if in == nil {
		return nil
	}
	out := new(RolloutConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RolloutPolicy) DeepCopyInto(out *RolloutPolicy) {
	*out = *in
	if in.TrafficRouting != nil {
		in, out := &in.TrafficRouting, &out.TrafficRouting
		*out = new(TrafficRoutingConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.TrafficAnalysis != nil {
		in, out := &in.TrafficAnalysis, &out.TrafficAnalysis
		*out = new(TrafficAnalysis)
		(*in).DeepCopyInto(*out)
	}
	if in.RolloutTimeoutSeconds != nil {
		in, out := &in.RolloutTimeoutSeconds, &out.RolloutTimeoutSeconds
		*out = new(int)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RolloutPolicy.
func (in *RolloutPolicy) DeepCopy() *RolloutPolicy {
	if in == nil {
		return nil
	}
	out := new(RolloutPolicy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RolloutStatus) DeepCopyInto(out *RolloutStatus) {
	*out = *in
	if in.RolloutStatusInCluster != nil {
		in, out := &in.RolloutStatusInCluster, &out.RolloutStatusInCluster
		*out = new(v1beta1.CanaryStatus)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RolloutStatus.
func (in *RolloutStatus) DeepCopy() *RolloutStatus {
	if in == nil {
		return nil
	}
	out := new(RolloutStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SessionAffinity) DeepCopyInto(out *SessionAffinity) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SessionAffinity.
func (in *SessionAffinity) DeepCopy() *SessionAffinity {
	if in == nil {
		return nil
	}
	out := new(SessionAffinity)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TrafficAnalysis) DeepCopyInto(out *TrafficAnalysis) {
	*out = *in
	if in.CheckIntervalSeconds != nil {
		in, out := &in.CheckIntervalSeconds, &out.CheckIntervalSeconds
		*out = new(int)
		**out = **in
	}
	if in.CheckFailedTimes != nil {
		in, out := &in.CheckFailedTimes, &out.CheckFailedTimes
		*out = new(int)
		**out = **in
	}
	if in.Metrics != nil {
		in, out := &in.Metrics, &out.Metrics
		*out = make([]Metric, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	in.Webhooks.DeepCopyInto(&out.Webhooks)
	if in.SessionAffinity != nil {
		in, out := &in.SessionAffinity, &out.SessionAffinity
		*out = new(SessionAffinity)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TrafficAnalysis.
func (in *TrafficAnalysis) DeepCopy() *TrafficAnalysis {
	if in == nil {
		return nil
	}
	out := new(TrafficAnalysis)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TrafficRoutingConfig) DeepCopyInto(out *TrafficRoutingConfig) {
	*out = *in
	if in.Gateways != nil {
		in, out := &in.Gateways, &out.Gateways
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Hosts != nil {
		in, out := &in.Hosts, &out.Hosts
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Retries != nil {
		in, out := &in.Retries, &out.Retries
		*out = new(v1alpha3.HTTPRetry)
		**out = **in
	}
	if in.Headers != nil {
		in, out := &in.Headers, &out.Headers
		*out = new(v1alpha3.Headers)
		(*in).DeepCopyInto(*out)
	}
	if in.CorsPolicy != nil {
		in, out := &in.CorsPolicy, &out.CorsPolicy
		*out = new(v1alpha3.CorsPolicy)
		(*in).DeepCopyInto(*out)
	}
	if in.CanaryStrategy != nil {
		in, out := &in.CanaryStrategy, &out.CanaryStrategy
		*out = new(CanaryConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.Match != nil {
		in, out := &in.Match, &out.Match
		*out = make([]v1alpha3.HTTPMatchRequest, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TrafficRoutingConfig.
func (in *TrafficRoutingConfig) DeepCopy() *TrafficRoutingConfig {
	if in == nil {
		return nil
	}
	out := new(TrafficRoutingConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Webhook) DeepCopyInto(out *Webhook) {
	*out = *in
	if in.TimeoutSeconds != nil {
		in, out := &in.TimeoutSeconds, &out.TimeoutSeconds
		*out = new(int)
		**out = **in
	}
	if in.Commands != nil {
		in, out := &in.Commands, &out.Commands
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Webhook.
func (in *Webhook) DeepCopy() *Webhook {
	if in == nil {
		return nil
	}
	out := new(Webhook)
	in.DeepCopyInto(out)
	return out
}
