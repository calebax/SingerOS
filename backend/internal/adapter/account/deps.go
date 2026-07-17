package account

import (
	"github.com/insmtx/Leros/backend/config"
	"github.com/insmtx/Leros/backend/internal/infra/sms"
	"github.com/insmtx/Leros/backend/internal/service"

	"gorm.io/gorm"
)

// Deps carries all dependencies that either adapter implementation might
// need. Fields only used by one variant are passed as nil by the other.
type Deps struct {
	DB                 *gorm.DB
	JWTSecret          string
	IAM                *config.IAMConfig
	WorkerProvisioning *service.WorkerProvisioningService
	// SmsSender is used only by the builtin (open-source) adapter to deliver
	// verification codes via infra/sms (Aliyun-backed). The enterprise adapter
	// delegates SMS to the IAM service and does not read this field.
	SmsSender  sms.SmsSender
	WorkerAuth *config.WorkerAuthConfig
}
