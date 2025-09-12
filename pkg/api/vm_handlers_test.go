package api

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/eliorerz/ovim-updated/pkg/kubevirt"
	"github.com/eliorerz/ovim-updated/pkg/models"
)

// MockVMProvisioner is a mock implementation of kubevirt.VMProvisioner interface
type MockVMProvisioner struct {
	mock.Mock
}

func (m *MockVMProvisioner) CreateVM(ctx context.Context, vm *models.VirtualMachine, vdc *models.VirtualDataCenter, template *models.Template) error {
	args := m.Called(ctx, vm, vdc, template)
	return args.Error(0)
}

func (m *MockVMProvisioner) DeleteVM(ctx context.Context, vmID string, namespace string) error {
	args := m.Called(ctx, vmID, namespace)
	return args.Error(0)
}

func (m *MockVMProvisioner) GetVMStatus(ctx context.Context, vmID string, namespace string) (*kubevirt.VMStatus, error) {
	args := m.Called(ctx, vmID, namespace)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kubevirt.VMStatus), args.Error(1)
}

func (m *MockVMProvisioner) StartVM(ctx context.Context, vmID string, namespace string) error {
	args := m.Called(ctx, vmID, namespace)
	return args.Error(0)
}

func (m *MockVMProvisioner) StopVM(ctx context.Context, vmID string, namespace string) error {
	args := m.Called(ctx, vmID, namespace)
	return args.Error(0)
}

func (m *MockVMProvisioner) RestartVM(ctx context.Context, vmID string, namespace string) error {
	args := m.Called(ctx, vmID, namespace)
	return args.Error(0)
}

func (m *MockVMProvisioner) GetVMIPAddress(ctx context.Context, vmID string, namespace string) (string, error) {
	args := m.Called(ctx, vmID, namespace)
	return args.String(0), args.Error(1)
}

func (m *MockVMProvisioner) CheckConnection(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func TestNewVMHandlers(t *testing.T) {
	mockStorage := &MockStorage{}
	mockProvisioner := &MockVMProvisioner{}

	handlers := NewVMHandlers(mockStorage, mockProvisioner, nil)

	assert.NotNil(t, handlers)
	assert.Equal(t, mockStorage, handlers.storage)
	assert.Equal(t, mockProvisioner, handlers.provisioner)
}
