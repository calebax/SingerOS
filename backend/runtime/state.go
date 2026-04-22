package runtime

import (
	runtimeeino "github.com/insmtx/SingerOS/backend/runtime/eino"
	runtimeevents "github.com/insmtx/SingerOS/backend/runtime/events"
	runtimeprompt "github.com/insmtx/SingerOS/backend/runtime/prompt"
)

type runState struct {
	req          *RequestContext
	emitter      *runtimeevents.Emitter
	userInput    string
	systemPrompt string
	toolBinding  runtimeeino.ToolBinding
	tools        *runtimeprompt.ToolsContext
	maxStep      int
}
