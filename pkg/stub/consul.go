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

func NewConsul(c *v1alpha1.Consul) error {
	// Create the bootstrap deployment if it doesn't exist
	// TODO deleted bootstrap will break existing cluster (cattle not pets)
	bootstrapDep := newConsulServerDeployment(c, true)
	err := action.Create(bootstrapDep)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create consul bootstap deployment: %v", err)
	}

	//Create the bootstrap k8s service
	bootstrapSvc := newConsulService(c, "bootstrap")
	err = action.Create(bootstrapSvc)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create consul bootstap service: %v", err)
	}

	// Create the server deployment
	serverDep := newConsulServerDeployment(c, false)
	err = action.Create(serverDep)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create consul bootstap deployment: %v", err)
	}

	//Create the bootstrap k8s service
	serverSvc := newConsulService(c, "server")
	err = action.Create(serverSvc)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create consul bootstap service: %v", err)
	}

	// Validate the size of the server cluster
	err = query.Get(serverDep)
	if err != nil {
		return fmt.Errorf("failed to get deployment: %v", err)
	}
	serverClusterSize := c.Spec.Server.Size - 1 // Exclude the bootstrap server
	if *serverDep.Spec.Replicas != serverClusterSize {
		serverDep.Spec.Replicas = &serverClusterSize
		err = action.Update(serverDep)
		if err != nil {
			return fmt.Errorf("failed to update deployment: %v", err)
		}
	}

	// Create the client deployment if it doesn't exist
	clientDep := newConsulClientDeployment(c)
	err = action.Create(clientDep)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create consul client deployment: %v", err)
	}

	// Validate the size of the client cluster
	err = query.Get(clientDep)
	if err != nil {
		return fmt.Errorf("failed to get deployment: %v", err)
	}
	clientSize := c.Spec.Client.Size
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
	err = query.List(c.Namespace, podList, query.WithListOptions(listOps))
	if err != nil {
		return fmt.Errorf("failed to list pods: %v", err)
	}
	podNames := getPodNames(podList.Items)
	if !reflect.DeepEqual(podNames, c.Status.Nodes) {
		c.Status.Nodes = podNames
		err := action.Update(c)
		if err != nil {
			return fmt.Errorf("failed to update consul status: %v", err)
		}
	}
	return nil
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