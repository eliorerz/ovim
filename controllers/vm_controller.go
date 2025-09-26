package controllers

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	ovimv1 "github.com/eliorerz/ovim-updated/pkg/api/v1"
	"github.com/eliorerz/ovim-updated/pkg/kubevirt"
	"github.com/eliorerz/ovim-updated/pkg/models"
	"github.com/eliorerz/ovim-updated/pkg/storage"
)

const (
	// VMFinalizer is the finalizer for VM resources
	VMFinalizer = "ovim.io/vm-finalizer"
)

// VMReconciler reconciles VM resources for VDCs
type VMReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	Storage        storage.Storage
	KubeVirtClient kubevirt.VMProvisioner
}

// VMRequest represents a VM creation/update request from the database
type VMRequest struct {
	Action   string // create, update, delete, start, stop, restart
	VM       *models.VirtualMachine
	VDC      *models.VirtualDataCenter
	Template *models.Template
}

// +kubebuilder:rbac:groups=kubevirt.io,resources=virtualmachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubevirt.io,resources=virtualmachineinstances,verbs=get;list;watch

// Reconcile handles VM lifecycle management
func (r *VMReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("vm-controller", req.NamespacedName)

	// Check if this is triggered by a VDC resource
	if req.NamespacedName.Name != "" && req.NamespacedName.Namespace != "" {
		// Fetch the VDC to check if it's spoke-managed
		var vdc ovimv1.VirtualDataCenter
		if err := r.Get(ctx, req.NamespacedName, &vdc); err != nil {
			if errors.IsNotFound(err) {
				return ctrl.Result{}, nil
			}
			logger.Error(err, "unable to fetch VirtualDataCenter")
			return ctrl.Result{}, err
		}

		// Hub VM controller should only process hub-managed VDCs, not spoke VDCs
		if managedBy, exists := vdc.Labels["ovim.io/managed-by"]; exists && managedBy == "spoke-agent" {
			logger.V(4).Info("Skipping spoke-managed VDC on hub VM controller", "vdc", vdc.Name, "managed-by", managedBy)
			return ctrl.Result{}, nil
		}
	}

	// This controller is triggered by VDC changes and database events
	// For now, we'll implement periodic reconciliation to sync VMs from database

	// Get all VMs that need reconciliation
	if r.Storage == nil {
		logger.V(1).Info("No storage configured, skipping VM reconciliation")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	vms, err := r.Storage.ListVMs("")
	if err != nil {
		logger.Error(err, "failed to list VMs from storage")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	for _, vm := range vms {
		if err := r.reconcileVM(ctx, vm); err != nil {
			logger.Error(err, "failed to reconcile VM", "vm", vm.Name, "vmId", vm.ID)
			// Continue with other VMs instead of failing completely
		}
	}

	return ctrl.Result{RequeueAfter: 2 * time.Minute}, nil
}

// reconcileVM handles individual VM reconciliation
func (r *VMReconciler) reconcileVM(ctx context.Context, vm *models.VirtualMachine) error {
	logger := log.FromContext(ctx).WithValues("vm", vm.Name, "vmId", vm.ID)

	// Get VDC information
	if vm.VDCID == nil {
		logger.V(1).Info("VM not assigned to VDC, skipping")
		return nil
	}

	vdc, err := r.Storage.GetVDC(*vm.VDCID)
	if err != nil {
		return fmt.Errorf("failed to get VDC %s: %w", *vm.VDCID, err)
	}

	if vdc.WorkloadNamespace == "" {
		logger.V(1).Info("VDC workload namespace not ready, skipping VM")
		return nil
	}

	// Get template information
	template, err := r.Storage.GetTemplate(vm.TemplateID)
	if err != nil {
		return fmt.Errorf("failed to get template %s: %w", vm.TemplateID, err)
	}

	// Check current state of VM in KubeVirt
	currentStatus, err := r.KubeVirtClient.GetVMStatus(ctx, vm.Name, vdc.WorkloadNamespace)
	if err != nil && !isNotFoundError(err) {
		return fmt.Errorf("failed to get VM status: %w", err)
	}

	vmExists := currentStatus != nil

	switch vm.Status {
	case "pending":
		if !vmExists {
			// Create the VM in KubeVirt
			if err := r.KubeVirtClient.CreateVM(ctx, vm, vdc, template); err != nil {
				return fmt.Errorf("failed to create VM in KubeVirt: %w", err)
			}
			logger.Info("Created VM in KubeVirt")

			// Update status in database
			vm.Status = "creating"
			if err := r.Storage.UpdateVM(vm); err != nil {
				logger.Error(err, "failed to update VM status in database")
			}
		}

	case "running":
		if vmExists {
			if currentStatus.Phase != "Running" {
				// Start the VM
				if err := r.KubeVirtClient.StartVM(ctx, vm.Name, vdc.WorkloadNamespace); err != nil {
					return fmt.Errorf("failed to start VM: %w", err)
				}
				logger.Info("Started VM")
			}
		} else {
			// VM should exist but doesn't - recreate it
			if err := r.KubeVirtClient.CreateVM(ctx, vm, vdc, template); err != nil {
				return fmt.Errorf("failed to recreate VM: %w", err)
			}
			logger.Info("Recreated missing VM")
		}

	case "stopped":
		if vmExists {
			if currentStatus.Phase == "Running" {
				// Stop the VM
				if err := r.KubeVirtClient.StopVM(ctx, vm.Name, vdc.WorkloadNamespace); err != nil {
					return fmt.Errorf("failed to stop VM: %w", err)
				}
				logger.Info("Stopped VM")
			}
		}

	case "deleted":
		if vmExists {
			// Delete the VM
			if err := r.KubeVirtClient.DeleteVM(ctx, vm.Name, vdc.WorkloadNamespace); err != nil {
				return fmt.Errorf("failed to delete VM: %w", err)
			}
			logger.Info("Deleted VM from KubeVirt")
		}
		// Remove from database
		if err := r.Storage.DeleteVM(vm.ID); err != nil {
			logger.Error(err, "failed to delete VM from database")
		}
	}

	// Update VM status and IP address from KubeVirt
	if vmExists {
		if currentStatus.IPAddress != "" && currentStatus.IPAddress != vm.IPAddress {
			vm.IPAddress = currentStatus.IPAddress
			if err := r.Storage.UpdateVM(vm); err != nil {
				logger.Error(err, "failed to update VM IP address in database")
			}
		}

		// Update status based on KubeVirt state
		var newStatus string
		switch currentStatus.Phase {
		case "Running":
			newStatus = "running"
		case "Stopped", "Succeeded":
			newStatus = "stopped"
		case "Pending", "Scheduling":
			newStatus = "creating"
		case "Failed":
			newStatus = "error"
		default:
			newStatus = "unknown"
		}

		if newStatus != vm.Status && vm.Status != "deleted" {
			vm.Status = newStatus
			if err := r.Storage.UpdateVM(vm); err != nil {
				logger.Error(err, "failed to update VM status in database")
			}
		}
	}

	return nil
}

// isNotFoundError checks if an error indicates a resource was not found
func isNotFoundError(err error) bool {
	return errors.IsNotFound(err) ||
		(err != nil && (fmt.Sprintf("%v", err) == "VirtualMachine not found" ||
			fmt.Sprintf("%v", err) == "virtualmachines.kubevirt.io not found"))
}

// SetupWithManager sets up the controller with the Manager
func (r *VMReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Initialize KubeVirt client if not provided
	if r.KubeVirtClient == nil {
		config := mgr.GetConfig()
		kvClient, err := kubevirt.NewClient(config, mgr.GetClient())
		if err != nil {
			return fmt.Errorf("failed to create KubeVirt client: %w", err)
		}
		r.KubeVirtClient = kvClient
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&ovimv1.VirtualDataCenter{}).
		Named("ovim-vm-controller").
		Complete(r)
}
