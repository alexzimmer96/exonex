package cortex

import (
	"context"
	"fmt"

	"github.com/alexzimmer96/exonex/pkg/storage"
	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Config struct {
	ServerAddr  string                  `mapstructure:"server-addr"`
	MetricsAddr string                  `mapstructure:"metrics-addr"`
	DatabaseUrl string                  `mapstructure:"database-url" validate:"required"`
	Volumes     map[string]VolumeConfig `mapstructure:"volumes" validate:"required"`
}

type VolumeConfig struct {
	Endpoint  string `mapstructure:"endpoint" validate:"required"`
	Region    string `mapstructure:"region" validate:"required"`
	Bucket    string `mapstructure:"bucket" validate:"required"`
	AccessKey string `mapstructure:"access-key" validate:"required"`
	SecretKey string `mapstructure:"secret-key" validate:"required"`
}

func (c Config) Validate() error {
	validate := validator.New()
	if err := validate.Struct(&c); err != nil {
		return err
	}
	return nil
}

func (c Config) GetDatabasePool() (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(context.Background(), c.DatabaseUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return pool, nil
}

func (c Config) GetVolumeManager() (*storage.VolumeManager, error) {
	if len(c.Volumes) == 0 {
		return nil, fmt.Errorf("no volumes found in configuration")
	}

	manager := storage.NewVolumeManager()
	for name, volume := range c.Volumes {
		err := manager.RegisterVolume(
			name,
			volume.Endpoint,
			volume.Region,
			volume.Bucket,
			volume.AccessKey,
			volume.SecretKey,
		)
		if err != nil {
			return nil, err
		}
	}

	return manager, nil
}
