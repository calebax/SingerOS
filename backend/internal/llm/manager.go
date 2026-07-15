package llm

import "context"

// Manager 定义统一的 LLM 模型配置管理接口。
// orgID 由调用方从认证上下文中解析后传入，Manager 实现不做认证。
type Manager interface {
	Create(ctx context.Context, orgID uint, req *CreateRequest) (*ModelConfig, error)
	Get(ctx context.Context, orgID uint, id uint, code string) (*ModelConfig, error)
	GetDefault(ctx context.Context, orgID uint) (*ModelConfig, error)
	Update(ctx context.Context, orgID uint, id uint, req *UpdateRequest) (*ModelConfig, error)
	Delete(ctx context.Context, orgID uint, id uint) error
	List(ctx context.Context, orgID uint, req *ListRequest) (*ListModelResult, error)
	TestConnectivity(ctx context.Context, orgID uint, req *TestRequest) (*TestResult, error)
}
