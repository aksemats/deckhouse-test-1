package deckhouse

import (
	"fmt"
	"k8s.io/apimachinery/pkg/api/errors"
	"strings"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"flant/deckhouse-candi/pkg/kubernetes/client"
	"flant/deckhouse-candi/pkg/log"
	"flant/deckhouse-candi/pkg/util/retry"
)

func DeleteDeckhouseDeployment(kubeCl *client.KubernetesClient) error {
	return retry.StartLoop("Delete Deckhouse", 45, 5, func() error {
		err := kubeCl.AppsV1().Deployments("d8-system").Delete("deckhouse", &metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
		return nil
	})
}

func DeleteStorageClasses(kubeCl *client.KubernetesClient) error {
	return retry.StartLoop("Delete StorageClasses", 45, 5, func() error {
		return kubeCl.StorageV1().StorageClasses().DeleteCollection(&metav1.DeleteOptions{}, metav1.ListOptions{})
	})
}

func DeletePods(kubeCl *client.KubernetesClient) error {
	return retry.StartLoop("Delete Pods", 45, 5, func() error {
		pods, err := kubeCl.CoreV1().Pods(metav1.NamespaceAll).List(metav1.ListOptions{})
		if err != nil {
			return err
		}

		for _, pod := range pods.Items {
			err := kubeCl.CoreV1().Pods(pod.Namespace).Delete(pod.Name, &metav1.DeleteOptions{})
			if err != nil {
				return err
			}
			log.InfoF("%s/%s\n", pod.Namespace, pod.Name)
		}

		return nil
	})
}

func DeleteServices(kubeCl *client.KubernetesClient) error {
	return retry.StartLoop("Delete Services", 45, 5, func() error {
		allServices, err := kubeCl.CoreV1().Services(metav1.NamespaceAll).List(metav1.ListOptions{})
		if err != nil {
			return err
		}

		for _, service := range allServices.Items {
			if service.Spec.Type != v1.ServiceTypeLoadBalancer {
				continue
			}

			err := kubeCl.CoreV1().Services(service.Namespace).Delete(service.Name, &metav1.DeleteOptions{})
			if err != nil {
				return err
			}
			log.InfoF("%s/%s\n", service.Namespace, service.Name)
		}
		return nil
	})
}

func DeletePV(kubeCl *client.KubernetesClient) error {
	return retry.StartLoop("Delete PersistentVolume", 45, 5, func() error {
		volumes, err := kubeCl.CoreV1().PersistentVolumes().List(metav1.ListOptions{})
		if err != nil {
			return err
		}

		for _, volume := range volumes.Items {
			err := kubeCl.CoreV1().PersistentVolumes().Delete(volume.Name, &metav1.DeleteOptions{})
			if err != nil {
				return err
			}
			log.InfoLn(volume.Name)
		}
		return nil
	})
}

func DeletePVC(kubeCl *client.KubernetesClient) error {
	return retry.StartLoop("Delete PersistentVolumeClaims", 45, 5, func() error {
		volumeClaims, err := kubeCl.CoreV1().PersistentVolumeClaims(metav1.NamespaceAll).List(metav1.ListOptions{})
		if err != nil {
			return err
		}

		for _, claim := range volumeClaims.Items {
			err := kubeCl.CoreV1().PersistentVolumeClaims(claim.Namespace).Delete(claim.Name, &metav1.DeleteOptions{})
			if err != nil {
				return err
			}
			log.InfoF("%s/%s\n", claim.Namespace, claim.Name)
		}
		return nil
	})
}

func DeleteMachineDeployments(kubeCl *client.KubernetesClient) error {
	machineDeploymentsSchema := schema.GroupVersionResource{Group: "machine.sapcloud.io", Version: "v1alpha1", Resource: "machinedeployments"}
	machinesSchema := schema.GroupVersionResource{Group: "machine.sapcloud.io", Version: "v1alpha1", Resource: "machines"}

	return retry.StartLoop("Delete MachineDeployments", 45, 5, func() error {
		allMachines, err := kubeCl.Dynamic().Resource(machinesSchema).Namespace(metav1.NamespaceAll).List(metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("get machines: %v", err)
		}

		for _, machine := range allMachines.Items {
			labels := machine.GetLabels()
			labels["force-deletion"] = "True"
			machine.SetLabels(labels)

			content, err := machine.MarshalJSON()
			if err != nil {
				return err
			}

			_, err = kubeCl.Dynamic().Resource(machinesSchema).Namespace(machine.GetNamespace()).Patch(machine.GetName(), types.MergePatchType, content, metav1.PatchOptions{})
			if err != nil {
				return fmt.Errorf("patch machine %s: %v", machine.GetName(), err)
			}
		}

		allMachineDeployments, err := kubeCl.Dynamic().Resource(machineDeploymentsSchema).Namespace(metav1.NamespaceAll).List(metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("get machinedeployments: %v", err)
		}

		for _, machineDeployment := range allMachineDeployments.Items {
			namespace := machineDeployment.GetNamespace()
			name := machineDeployment.GetName()
			err := kubeCl.Dynamic().Resource(machineDeploymentsSchema).Namespace(namespace).Delete(name, &metav1.DeleteOptions{})
			if err != nil {
				return fmt.Errorf("delete machinedeployments %s: %v", name, err)
			}
			log.InfoF("%s/%s\n", namespace, name)
		}
		return nil
	})
}

func WaitForMachinesDeletion(kubeCl *client.KubernetesClient) error {
	resourceSchema := schema.GroupVersionResource{Group: "machine.sapcloud.io", Version: "v1alpha1", Resource: "machines"}
	return retry.StartLoop("Wait for Machines deletion", 45, 15, func() error {
		resources, err := kubeCl.Dynamic().Resource(resourceSchema).List(metav1.ListOptions{})
		if err != nil {
			return err
		}

		count := len(resources.Items)
		if count != 0 {
			builder := strings.Builder{}
			for _, item := range resources.Items {
				builder.WriteString(fmt.Sprintf("\t\t%s/%s\n", item.GetNamespace(), item.GetName()))
			}
			return fmt.Errorf("%d Machines left in the cluster\n%s", count, strings.TrimSuffix(builder.String(), "\n"))
		}
		log.InfoLn("All Machines are deleted from the cluster")
		return nil
	})
}

func WaitForServicesDeletion(kubeCl *client.KubernetesClient) error {
	return retry.StartLoop("Wait for Services deletion", 45, 15, func() error {
		resources, err := kubeCl.CoreV1().Services(metav1.NamespaceAll).List(metav1.ListOptions{})
		if err != nil {
			return err
		}

		var filteredResources []v1.Service
		for _, resource := range resources.Items {
			if resource.Spec.Type == v1.ServiceTypeLoadBalancer {
				filteredResources = append(filteredResources, resource)
			}
		}

		count := len(filteredResources)
		if count != 0 {
			builder := strings.Builder{}
			for _, item := range filteredResources {
				builder.WriteString(fmt.Sprintf("\t\t%s/%s\n", item.Namespace, item.Name))
			}
			return fmt.Errorf("%d Services left in the cluster\n%s", count, strings.TrimSuffix(builder.String(), "\n"))
		}
		log.InfoLn("All Services with type LoadBalancer are deleted from the cluster")
		return nil
	})
}

func WaitForPVDeletion(kubeCl *client.KubernetesClient) error {
	return retry.StartLoop("Wait for PersistentVolumes deletion", 45, 15, func() error {
		resources, err := kubeCl.CoreV1().PersistentVolumes().List(metav1.ListOptions{})
		if err != nil {
			return err
		}

		count := len(resources.Items)
		if count != 0 {
			builder := strings.Builder{}
			for _, item := range resources.Items {
				builder.WriteString(fmt.Sprintf("\t\t%s | %s\n", item.Name, item.Status.Phase))
			}
			return fmt.Errorf("%d PersistentVolumes left in the cluster\n%s", count, strings.TrimSuffix(builder.String(), "\n"))
		}
		log.InfoLn("All PersistentVolumes are deleted from the cluster")
		return nil
	})
}

func WaitForPVCDeletion(kubeCl *client.KubernetesClient) error {
	return retry.StartLoop("Wait for PersistentVolumeClaims deletion", 45, 15, func() error {
		resources, err := kubeCl.CoreV1().PersistentVolumeClaims(metav1.NamespaceAll).List(metav1.ListOptions{})
		if err != nil {
			return err
		}

		// Pending PVCs have no attached PVs, we have to skip them
		var filteredResources []v1.PersistentVolumeClaim
		for _, resource := range resources.Items {
			if resource.Status.Phase != v1.ClaimPending {
				filteredResources = append(filteredResources, resource)
			}
		}

		count := len(filteredResources)
		if count != 0 {
			builder := strings.Builder{}
			for _, item := range resources.Items {
				builder.WriteString(fmt.Sprintf("\t\t%s | %s\n", item.Name, item.Status.Phase))
			}
			return fmt.Errorf("%d PersistentVolumeClaims left in the cluster\n%s", count, strings.TrimSuffix(builder.String(), "\n"))
		}
		log.InfoLn("All PersistentVolumeClaims are deleted from the cluster")
		return nil
	})
}
