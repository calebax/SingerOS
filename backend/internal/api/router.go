package api

import "github.com/gin-gonic/gin"

// Router manages connector routing registration
type Router struct {
	connectors map[string]Connector
}

// NewRouter creates a new connector router
func NewRouter() *Router {
	return &Router{
		connectors: make(map[string]Connector),
	}
}

// Register registers a connector
func (r *Router) Register(c Connector) {
	r.connectors[c.ChannelCode()] = c
}

// RegisterRoutes registers routes for all connectors
func (r *Router) RegisterRoutes(router gin.IRouter) {
	for _, c := range r.connectors {
		c.RegisterRoutes(router)
	}
}
