package controllers

import (
	"github.com/stretchr/testify/assert"
	"kurator.dev/kurator/pkg/apis/cluster/v1alpha1"
	"testing"
)

func TestGetHostsContent(t *testing.T) {

	master1 := v1alpha1.Machine{
		HostName:  "master1",
		PrivateIP: "1.1.1.1",
		PublicIP:  "2.2.2.2",
	}
	node1 := v1alpha1.Machine{
		HostName:  "node1",
		PrivateIP: "3.3.3.3",
		PublicIP:  "4.4.4.4",
	}

	curCustomMachine := &v1alpha1.CustomMachine{
		Spec: v1alpha1.CustomMachineSpec{
			Master: []v1alpha1.Machine{master1},
			Nodes:  []v1alpha1.Machine{node1},
		},
	}

	expectHost := &HostTemplateContent{
		NodeAndIP:    []string{"master1 ansible_host=2.2.2.2 ip=1.1.1.1", "node1 ansible_host=4.4.4.4 ip=3.3.3.3"},
		MasterName:   []string{"master1"},
		NodeName:     []string{"node1"},
		EtcdNodeName: []string{"master1"},
	}
	assert.Equal(t, expectHost, GetHostsContent(curCustomMachine))

}
