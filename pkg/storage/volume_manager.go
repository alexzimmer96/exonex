package storage

import (
	"errors"
	"fmt"
)

var (
	ErrVolumeNameAlreadyExists = errors.New("volume name already exists")
	ErrVolumeNameNotRegistered = errors.New("volume name not registered")
)

// A VolumeManager makes a set of Adapters available under a unique registry and identifies them by a name.
type VolumeManager struct {
	volumes map[string]*Adapter
}

// NewVolumeManager creates a new empty VolumeManager instance.
func NewVolumeManager() *VolumeManager {
	return &VolumeManager{
		volumes: make(map[string]*Adapter),
	}
}

// RegisterVolume registers a new Adapter under the given name used the provided configuration.
// Returns an ErrVolumeNameAlreadyExists error when  the name is already registered.
func (vm *VolumeManager) RegisterVolume(name, endpoint, region, bucket, accessKey, secretKey string) error {
	if _, exists := vm.volumes[name]; exists {
		return fmt.Errorf("%w: %s", ErrVolumeNameAlreadyExists, name)
	}
	vm.volumes[name] = NewAdapter(endpoint, region, bucket, secretKey, accessKey)
	return nil
}

// GetAdapter returns an Adapter for a registered volume.
// Returns an ErrVolumeNameNotRegistered if the volume name has not been registered before.
func (vm *VolumeManager) GetAdapter(name string) (*Adapter, error) {
	adapter, exists := vm.volumes[name]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrVolumeNameNotRegistered, name)
	}
	return adapter, nil
}
