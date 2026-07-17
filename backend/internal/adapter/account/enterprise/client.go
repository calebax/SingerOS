//go:build enterprise

package enterprise

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/insmtx/Leros/backend/config"
)

// iamClient calls IAM service HTTP APIs for user authentication and
// token verification.
type iamClient struct {
	baseURL    string
	httpClient *http.Client
}

func newIAMClient(cfg *config.IAMConfig) *iamClient {
	baseURL := ""
	if cfg != nil {
		baseURL = strings.TrimRight(cfg.BaseURL, "/")
	}
	return &iamClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

// doRequest constructs a request to the IAM service.
func (c *iamClient) doRequest(ctx context.Context, method, path string) (*http.Response, error) {
	url := fmt.Sprintf("%s/%s", c.baseURL, strings.TrimLeft(path, "/"))
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create iam request: %w", err)
	}
	return c.httpClient.Do(req)
}
