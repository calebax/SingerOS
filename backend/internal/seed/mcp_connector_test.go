package seed

import (
	"testing"

	"github.com/insmtx/Leros/backend/config"
	"github.com/insmtx/Leros/backend/types"
)

func TestConfiguredMCPConnectorSpecsMapsDefaultsAndAuthorization(t *testing.T) {
	specs := configuredMCPConnectorSpecs([]config.MCPConnectorConfig{
		{
			Channel: "example", Name: "Example",
			Bindings: config.MCPConnectorBindings{MCPBearerToken: "token"},
			Auth: config.MCPConnectorAuthConfig{
				Type: "form",
				Fields: []config.MCPConnectorAuthField{{
					Key: "token", Label: "Token", Type: "password", Required: true,
				}},
			},
		},
	})
	if len(specs) != 1 {
		t.Fatalf("specs = %#v", specs)
	}
	spec := specs[0]
	if spec.Status != types.MCPChannelStatusActive || spec.AuthType != types.MCPChannelAuthTypeForm {
		t.Fatalf("defaults = %#v", spec)
	}
	if len(spec.AuthConfig.Fields) != 1 || spec.AuthConfig.Fields[0].Key != "token" ||
		spec.AuthConfig.Bindings.MCPBearerToken != "token" {
		t.Fatalf("authorization = %#v", spec.AuthConfig)
	}
}
