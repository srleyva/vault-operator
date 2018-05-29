package stub

import (
	"k8s.io/api/core/v1"
	"github.com/srleyva/vault-operator/pkg/apis/vault/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"github.com/operator-framework/operator-sdk/pkg/sdk/action"
	"fmt"
	"github.com/operator-framework/operator-sdk/pkg/sdk/query"
	"k8s.io/apimachinery/pkg/labels"
	"reflect"
)

func NewVault(v *v1alpha1.Vault) error {
	vaultDep := newVaultDeployment(v)
	err := action.Create(vaultDep)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create vault deployment: %v", err)
	}

	vaultSvc := newvaultService(v)
	err = action.Create(vaultSvc)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create vault service: %v", err)
	}

	// Validate the size of the server cluster
	err = query.Get(vaultDep)
	if err != nil {
		return fmt.Errorf("failed to get deployment: %v", err)
	}
	nodeClusterSize := v.Spec.Size
	if *vaultDep.Spec.Replicas != nodeClusterSize {
		vaultDep.Spec.Replicas = &nodeClusterSize
		err = action.Update(vaultDep)
		if err != nil {
			return fmt.Errorf("failed to update deployment: %v", err)
		}
	}

	// Update the vault pod list
	podList := podList()
	labelSelector := labels.SelectorFromSet(labelsForVault()).String()
	listOps := &metav1.ListOptions{LabelSelector: labelSelector}
	err = query.List(v.Namespace, podList, query.WithListOptions(listOps))
	if err != nil {
		return fmt.Errorf("failed to list pods: %v", err)
	}
	podNames := getPodNames(podList.Items)
	if !reflect.DeepEqual(podNames, v.Status.Nodes) {
		v.Status.Nodes = podNames
		err := action.Update(v)
		if err != nil {
			return fmt.Errorf("failed to update consul status: %v", err)
		}
	}
	return nil
}

func newVaultDeployment(v *v1alpha1.Vault) *appsv1.Deployment {
	replicas := v.Spec.Size
	ls := labelsForVault()
	dep := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1beta1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      v.Name,
			Namespace: v.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: ls},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: ls},
				Spec: v1.PodSpec{
					Containers: []v1.Container{{
						Image: "sleyva97/vault",
						Name:  "vault-node",
						Command: []string{"vault", "server", "-config", "/etc/config.hcl"},
						Ports: []v1.ContainerPort{
							{ContainerPort: 8200, Name: "vault"},
						},
					},
						{
							Image:   "consul:1.1.0",
							Name:    "consul-client",
							Env: []v1.EnvVar{
								{
									Name:  "CONSUL_LOCAL_CONFIG",
									Value: "{\"leave_on_terminate\": true}",
								},
							},
							Command: []string{"consul", "agent", "-data-dir", "/tmp/consul", "-retry-join", "consul-server:8301"},
							Ports: []v1.ContainerPort{{
								ContainerPort: 8500,
								Name:          "consul"}},
						}},
				},
			},
		},
	}
	addOwnerRefToObject(dep, asOwnerVault(v))
	return dep
}

func labelsForVault() map[string]string {
	return map[string]string{"app": "vault"}
}

func newvaultService(v *v1alpha1.Vault) *v1.Service {
	selector := labelsForVault()
	svc := &v1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      v.GetName(),
			Namespace: v.GetNamespace(),
			Labels:    selector,
		},
		Spec: v1.ServiceSpec{
			Selector: selector,
			Ports: []v1.ServicePort{
				{
					Name:     "vault",
					Protocol: v1.ProtocolTCP,
					Port:     8200,
				},
			},
		},
	}
	addOwnerRefToObject(svc, asOwnerVault(v))
	return svc
}

// asOwner returns an OwnerReference set as the memcached CR
func asOwnerVault(v *v1alpha1.Vault) metav1.OwnerReference {
	trueVar := true
	return metav1.OwnerReference{
		APIVersion: v.APIVersion,
		Kind:       v.Kind,
		Name:       v.Name,
		UID:        v.UID,
		Controller: &trueVar,
	}
}
