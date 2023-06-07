package exposure

import "context"

type ObservableExposureProvisioner interface {
	CreateExposure(ctx context.Context, exposure Exposure) error
	DeleteExposure(ctx context.Context, exposure Exposure) error
	ListExposure(ctx context.Context) ([]Exposure, error)
}
