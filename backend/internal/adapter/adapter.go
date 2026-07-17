package adapter

import (
	"gorm.io/gorm"

	"github.com/insmtx/Leros/backend/config"
	"github.com/insmtx/Leros/backend/internal/adapter/account"
	"github.com/insmtx/Leros/backend/internal/infra/sms"
	"github.com/insmtx/Leros/backend/internal/service"
)

// Config aggregates all dependencies that either edition might need.
// Fields only used by one variant are passed as nil by the other.
type Config struct {
	DB                 *gorm.DB
	JWTSecret          string
	IAM                *config.IAMConfig
	WorkerProvisioning *service.WorkerProvisioningService
	SmsSender          sms.SmsSender
	WorkerAuth         *config.WorkerAuthConfig
}

// ToDeps converts the adapter Config to account Deps.
func (c Config) ToDeps() account.Deps {
	return account.Deps{
		DB:                 c.DB,
		JWTSecret:          c.JWTSecret,
		IAM:                c.IAM,
		WorkerProvisioning: c.WorkerProvisioning,
		SmsSender:          c.SmsSender,
		WorkerAuth:         c.WorkerAuth,
	}
}
