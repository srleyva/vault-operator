package stub

import (
	"github.com/srleyva/vault-operator/pkg/apis/vault/v1alpha1"

	"fmt"
	"github.com/operator-framework/operator-sdk/pkg/sdk/action"
	"github.com/operator-framework/operator-sdk/pkg/sdk/handler"
	"github.com/operator-framework/operator-sdk/pkg/sdk/query"
	"github.com/operator-framework/operator-sdk/pkg/sdk/types"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"reflect"
)

func NewHandler() handler.Handler {
	return &Handler{}
}

type Handler struct {
	// Fill me
}

func (h *Handler) Handle(ctx types.Context, event types.Event) error {
	switch o := event.Object.(type) {
	case *v1alpha1.Vault:
		vault := o
		// Ignore the delete event since the garbage collector will clean up all secondary resources for the CR
		// All secondary resources must have the CR set as their OwnerReference for this to be the case
		if event.Deleted {
			return nil
		}

		vaultDep := newVaultDeployment(vault)
		err := action.Create(vaultDep)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create vault deployment: %v", err)
		}

		vaultSvc := newvaultService(vault)
		err = action.Create(vaultSvc)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create vault service: %v", err)
		}

		// Validate the size of the server cluster
		err = query.Get(vaultDep)
		if err != nil {
			return fmt.Errorf("failed to get deployment: %v", err)
		}
		nodeClusterSize := vault.Spec.Size
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
		err = query.List(vault.Namespace, podList, query.WithListOptions(listOps))
		if err != nil {
			return fmt.Errorf("failed to list pods: %v", err)
		}
		podNames := getPodNames(podList.Items)
		if !reflect.DeepEqual(podNames, vault.Status.Nodes) {
			vault.Status.Nodes = podNames
			err := action.Update(vault)
			if err != nil {
				return fmt.Errorf("failed to update consul status: %v", err)
			}
		}
	case *v1alpha1.Consul:
		consul := o

		// Ignore the delete event since the garbage collector will clean up all secondary resources for the CR
		// All secondary resources must have the CR set as their OwnerReference for this to be the case
		if event.Deleted {
			return nil
		}

		// Create the bootstrap deployment if it doesn't exist
		// TODO deleted bootstrap will break existing cluster (cattle not pets)
		bootstrapDep := newConsulServerDeployment(consul, true)
		err := action.Create(bootstrapDep)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create consul bootstap deployment: %v", err)
		}

		//Create the bootstrap k8s service
		bootstrapSvc := newConsulService(consul, "bootstrap")
		err = action.Create(bootstrapSvc)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create consul bootstap service: %v", err)
		}

		// Create the server deployment
		serverDep := newConsulServerDeployment(consul, false)
		err = action.Create(serverDep)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create consul bootstap deployment: %v", err)
		}

		//Create the bootstrap k8s service
		serverSvc := newConsulService(consul, "server")
		err = action.Create(serverSvc)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create consul bootstap service: %v", err)
		}

		// Validate the size of the server cluster
		err = query.Get(serverDep)
		if err != nil {
			return fmt.Errorf("failed to get deployment: %v", err)
		}
		serverClusterSize := consul.Spec.Server.Size - 1 // Exclude the bootstrap server
		if *serverDep.Spec.Replicas != serverClusterSize {
			serverDep.Spec.Replicas = &serverClusterSize
			err = action.Update(serverDep)
			if err != nil {
				return fmt.Errorf("failed to update deployment: %v", err)
			}
		}

		// Create the client deployment if it doesn't exist
		clientDep := newConsulClientDeployment(consul)
		err = action.Create(clientDep)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create consul client deployment: %v", err)
		}

		// Validate the size of the client cluster
		err = query.Get(clientDep)
		if err != nil {
			return fmt.Errorf("failed to get deployment: %v", err)
		}
		clientSize := consul.Spec.Client.Size
		if *clientDep.Spec.Replicas != clientSize {
			clientDep.Spec.Replicas = &clientSize
			err = action.Update(clientDep)
			if err != nil {
				return fmt.Errorf("failed to update deployment: %v", err)
			}
		}

		// Update the consul pod list
		podList := podList()
		labelSelector := labels.SelectorFromSet(labelsForConsul("")).String()
		listOps := &metav1.ListOptions{LabelSelector: labelSelector}
		err = query.List(consul.Namespace, podList, query.WithListOptions(listOps))
		if err != nil {
			return fmt.Errorf("failed to list pods: %v", err)
		}
		podNames := getPodNames(podList.Items)
		if !reflect.DeepEqual(podNames, consul.Status.Nodes) {
			consul.Status.Nodes = podNames
			err := action.Update(consul)
			if err != nil {
				return fmt.Errorf("failed to update consul status: %v", err)
			}
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

func newConsulServerDeployment(c *v1alpha1.Consul, bootstrap bool) *appsv1.Deployment {
	replicas := c.Spec.Server.Size - 1
	command := []string{"consul", "agent", "-server", "-data-dir", "/tmp/consul", "-retry-join", "consul-bootstrap:8301"}
	name := c.Name + "-server"
	ls := labelsForConsul("server")
	grace := int64(60)
	if bootstrap {
		replicas = 1
		command = []string{"consul", "agent", "-server", "-bootstrap-expect", "1", "-data-dir", "/tmp/consul", "-client", "0.0.0.0"}
		name = c.Name + "-bootstrap"
		ls = labelsForConsul("bootstrap")

	}

	dep := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1beta1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: c.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: ls},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: ls},
				Spec: v1.PodSpec{
					TerminationGracePeriodSeconds: &grace,
					Containers: []v1.Container{{
						Image: "consul:1.1.0",
						Name:  "consul-server",
						Env: []v1.EnvVar{
							{
								Name:  "CONSUL_LOCAL_CONFIG",
								Value: "{\"leave_on_terminate\": true}",
							},
						},
						Command: command,
						Ports: []v1.ContainerPort{
							{ContainerPort: 8500, Name: "consul"},
							{ContainerPort: 8301, Name: "consul-cluster"},
						},
					}},
				},
			},
		},
	}
	addOwnerRefToObject(dep, asOwnerConsul(c))
	return dep
}

func newConsulService(c *v1alpha1.Consul, consulType string) *v1.Service {
	selector := labelsForConsul(consulType)
	svc := &v1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.GetName() + "-" + consulType,
			Namespace: c.GetNamespace(),
			Labels:    selector,
		},
		Spec: v1.ServiceSpec{
			Selector: selector,
			Ports: []v1.ServicePort{
				{
					Name:     "consul-dns",
					Protocol: v1.ProtocolTCP,
					Port:     8600,
				},
				{
					Name:     "consul-http",
					Protocol: v1.ProtocolTCP,
					Port:     8500,
				},
				{
					Name:     "consul-cluster",
					Protocol: v1.ProtocolTCP,
					Port:     8301,
				},
			},
		},
	}
	addOwnerRefToObject(svc, asOwnerConsul(c))
	return svc
}

func newConsulClientDeployment(cr *v1alpha1.Consul) *appsv1.Deployment {
	replicas := cr.Spec.Client.Size
	ls := labelsForConsul("Client")
	dep := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1beta1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name + "-client",
			Namespace: cr.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: ls},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: ls},
				Spec: v1.PodSpec{
					Containers: []v1.Container{{
						Image: "consul:1.1.0",
						Name:  "consul-client",
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
	addOwnerRefToObject(dep, asOwnerConsul(cr))
	return dep
}

func labelsForConsul(consulType string) map[string]string {

	if consulType == "" {
		return map[string]string{"app": "consul"}
	}
	return map[string]string{"app": "consul", "type": consulType}
}

// addOwnerRefToObject appends the desired OwnerReference to the object
func addOwnerRefToObject(obj metav1.Object, ownerRef metav1.OwnerReference) {
	obj.SetOwnerReferences(append(obj.GetOwnerReferences(), ownerRef))
}


// TODO These need to be combined
// asOwner returns an OwnerReference set as the consul CR
func asOwnerConsul(c *v1alpha1.Consul) metav1.OwnerReference {
	trueVar := true
	return metav1.OwnerReference{
		APIVersion: c.APIVersion,
		Kind:       c.Kind,
		Name:       c.Name,
		UID:        c.UID,
		Controller: &trueVar,
	}
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

// getPodNames returns the pod names of the array of pods passed in
func getPodNames(pods []v1.Pod) []string {
	var podNames []string
	for _, pod := range pods {
		podNames = append(podNames, pod.Name)
	}
	return podNames
}

// podList returns a v1.PodList object
func podList() *v1.PodList {
	return &v1.PodList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
	}
}
