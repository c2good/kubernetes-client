/**
 * Copyright (C) 2015 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *         http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package in_pod

import (
	"errors"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kcontainer "k8s.io/kubernetes/pkg/kubelet/container"
	kexec "k8s.io/utils/exec"

	"github.com/openshift/origin/pkg/network"
	networktypedclient "github.com/openshift/origin/pkg/network/generated/internalclientset/typed/network/internalversion"
	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/cluster/network/in_pod/util"
	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/types"
)

const (
	CheckPodNetworkName = "CheckPodNetwork"
)

// CheckPodNetwork is a Diagnostic to check communication between pods in the cluster.
type CheckPodNetwork struct {
	KubeClient           kclientset.Interface
	NetNamespacesClient  networktypedclient.NetNamespacesGetter
	ClusterNetworkClient networktypedclient.ClusterNetworksGetter

	vnidMap map[string]uint32
	res     types.DiagnosticResult
}

// Name is part of the Diagnostic interface and just returns name.
func (d CheckPodNetwork) Name() string {
	return CheckPodNetworkName
}

// Description is part of the Diagnostic interface and just returns the diagnostic description.
func (d CheckPodNetwork) Description() string {
	return "Check pod to pod communication in the cluster. In case of ovs-subnet network plugin, all pods should be able to communicate with each other and in case of multitenant network plugin, pods in non-global projects should be isolated and pods in global projects should be able to access any pod in the cluster and vice versa."
}

func (d CheckPodNetwork) Requirements() (client bool, host bool) {
	return true, false
}

// CanRun is part of the Diagnostic interface; it determines if the conditions are right to run this diagnostic.
func (d CheckPodNetwork) CanRun() (bool, error) {
	if d.KubeClient == nil {
		return false, errors.New("must have kube client")
	} else if d.NetNamespacesClient == nil || d.ClusterNetworkClient == nil {
		return false, errors.New("must have openshift client")
	}
	return true, nil
}

// Check is part of the Diagnostic interface; it runs the actual diagnostic logic
func (d CheckPodNetwork) Check() types.DiagnosticResult {
	d.res = types.NewDiagnosticResult(CheckPodNetworkName)

	pluginName, ok, err := util.GetOpenShiftNetworkPlugin(d.ClusterNetworkClient)
	if err != nil {
		d.res.Error("DPodNet1001", err, fmt.Sprintf("Checking network plugin failed. Error: %s", err))
		return d.res
	}
	if !ok {
		d.res.Warn("DPodNet1002", nil, "Skipping pod connectivity test. Reason: Not using openshift network plugin.")
		return d.res
	}

	localPods, nonlocalPods, err := util.GetLocalAndNonLocalDiagnosticPods(d.KubeClient)
	if err != nil {
		d.res.Error("DPodNet1003", err, fmt.Sprintf("Getting local and nonlocal pods failed. Error: %s", err))
		return d.res
	}

	if network.IsOpenShiftMultitenantNetworkPlugin(pluginName) {
		netnsList, err := d.NetNamespacesClient.NetNamespaces().List(metav1.ListOptions{})
		if err != nil {
			d.res.Error("DPodNet1004", err, fmt.Sprintf("Getting all network namespaces failed. Error: %s", err))
			return d.res
		}

		d.vnidMap = map[string]uint32{}
		for _, netns := range netnsList.Items {
			d.vnidMap[netns.NetName] = netns.NetID
		}
	}

	localGlobalPods, localNonGlobalPods := util.GetGlobalAndNonGlobalPods(localPods, d.vnidMap)
	nonlocalGlobalPods, nonlocalNonGlobalPods := util.GetGlobalAndNonGlobalPods(nonlocalPods, d.vnidMap)

	d.checkSameNodePodToPodConnection(localGlobalPods, localNonGlobalPods)
	d.checkDifferentNodePodToPodConnection(localGlobalPods, localNonGlobalPods, nonlocalGlobalPods, nonlocalNonGlobalPods)
	return d.res
}

func (d CheckPodNetwork) checkDifferentNodePodToPodConnection(localGlobalPods, localNonGlobalPods, nonlocalGlobalPods, nonlocalNonGlobalPods []kapi.Pod) {
	// Applicable to flat and multitenant networks
	d.checkConnection(localGlobalPods, nonlocalGlobalPods, "Skipping pod connectivity test for global projects on different nodes. Reason: Couldn't find 2 global pods.", true)

	// Applicable to multitenant network
	isMultitenant := (d.vnidMap != nil)
	if isMultitenant {
		d.checkConnection(localNonGlobalPods, nonlocalNonGlobalPods, "Skipping pod connectivity test for non-global projects on different nodes. Reason: Couldn't find 2 non-global pods.", true)

		d.checkConnection(localGlobalPods, nonlocalNonGlobalPods, "Skipping pod connectivity test between global to non-global projects on different nodes. Reason: Couldn't find global and/or non-global pod.", false)
	}
}

func (d CheckPodNetwork) checkSameNodePodToPodConnection(globalPods, nonGlobalPods []kapi.Pod) {
	// Applicable to both flat and multitenant networks
	d.checkConnection(globalPods, globalPods, "Skipping pod connectivity test for global projects on the same node. Reason: Couldn't find 2 global pods.", true)

	// Applicable to multitenant network
	isMultitenant := (d.vnidMap != nil)
	if isMultitenant {
		d.checkConnection(nonGlobalPods, nonGlobalPods, "Skipping pod connectivity test for non-global projects on the same node. Reason: Couldn't find 2 non-global pods.", true)

		d.checkConnection(globalPods, nonGlobalPods, "Skipping pod connectivity test between global to non-global projects on the same node. Reason: Couldn't find global and/or non-global pod.", false)
	}
}

func (d CheckPodNetwork) checkConnection(pods1, pods2 []kapi.Pod, warnMsg string, checkSameNamespace bool) {
	minCount := 1
	if len(pods1) > 0 && len(pods2) > 0 && (pods1[0].UID == pods2[0].UID) {
		minCount += 1
	}
	if len(pods1) < minCount || len(pods2) < minCount {
		d.res.Warn("DPodNet1005", nil, warnMsg)
		return
	}

	sameNamespace := false
	diffNamespace := false

	// Test pod to pod connection between same and different namespaces
	for _, pod1 := range pods1 {
		for _, pod2 := range pods2 {
			if sameNamespace && diffNamespace {
				return
			}
			if pod1.UID == pod2.UID {
				continue
			}
			if !sameNamespace && (pod1.Namespace == pod2.Namespace) {
				sameNamespace = true
				d.checkPodToPodConnection(&pod1, &pod2)
			}
			if !diffNamespace && (pod1.Namespace != pod2.Namespace) {
				diffNamespace = true
				d.checkPodToPodConnection(&pod1, &pod2)
			}
		}
	}

	if checkSameNamespace && !sameNamespace {
		d.res.Warn("DPodNet1010", nil, fmt.Sprintf("Same Namespace: %s", warnMsg))
	}
	if !diffNamespace {
		d.res.Warn("DPodNet1011", nil, fmt.Sprintf("Different namespaces: %s", warnMsg))
	}
}

// checkPodToPodConnection verifies connection from fromPod to toPod.
// Connection check from toPod to fromPod will be done by the node of toPod.
func (d CheckPodNetwork) checkPodToPodConnection(fromPod, toPod *kapi.Pod) {
	if len(fromPod.Status.ContainerStatuses) == 0 {
		err := fmt.Errorf("ContainerID not found for pod %q", util.PrintPod(fromPod))
		d.res.Error("DPodNet1006", err, err.Error())
		return
	}

	success := util.ExpectedConnectionStatus(fromPod.Namespace, toPod.Namespace, d.vnidMap)

	kexecer := kexec.New()
	containerID := kcontainer.ParseContainerID(fromPod.Status.ContainerStatuses[0].ContainerID).ID
	pid, err := kexecer.Command("docker", "inspect", "-f", "{{.State.Pid}}", containerID).CombinedOutput()
	if err != nil {
		d.res.Error("DPodNet1007", err, fmt.Sprintf("Fetching pid for pod %q, container %q failed. Error: %s", util.PrintPod(fromPod), containerID, err))
		return
	}

	out, err := kexecer.Command("nsenter", "-n", "-t", strings.Trim(fmt.Sprintf("%s", pid), "\n"), "--", "ping", "-c1", "-W2", toPod.Status.PodIP).CombinedOutput()
	if success && err != nil {
		d.res.Error("DPodNet1008", err, fmt.Sprintf("Connectivity from pod %q to pod %q failed. Error: %s, Out: %s", util.PrintPod(fromPod), util.PrintPod(toPod), err, string(out)))
	} else if !success && err == nil {
		msg := fmt.Sprintf("Unexpected connectivity from pod %q to pod %q.", util.PrintPod(fromPod), util.PrintPod(toPod))
		d.res.Error("DPodNet1009", fmt.Errorf("%s", msg), msg)
	}
}
