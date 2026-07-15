package service

import (
	"gorm.io/gorm"

	"github.com/insmtx/Leros/backend/internal/api/contract"
	"github.com/insmtx/Leros/backend/internal/llm"
)

// NewLLMModelService 创建 LLM 模型配置服务，委托到 llm.Manager。
func NewLLMModelService(db *gorm.DB) contract.LLMModelService {
	return llm.NewContractAdapter(llm.NewManager(db))
}
