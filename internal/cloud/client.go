package cloud

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const baseURL = "https://api.clickhouse.cloud/v1"

type Client struct {
	apiKey    string
	apiSecret string
	http      *http.Client
}

func NewClient(apiKey, apiSecret string) *Client {
	return &Client{
		apiKey:    apiKey,
		apiSecret: apiSecret,
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) authHeader() string {
	creds := base64.StdEncoding.EncodeToString([]byte(c.apiKey + ":" + c.apiSecret))
	return "Basic " + creds
}

type apiResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *apiError       `json:"error,omitempty"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (c *Client) do(method, path string, body string) (json.RawMessage, error) {
	url := baseURL + path

	var reqBody io.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", c.authHeader())
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiResp apiResponse
		if json.Unmarshal(data, &apiResp) == nil && apiResp.Error != nil {
			return nil, fmt.Errorf("API error %s: %s", apiResp.Error.Code, apiResp.Error.Message)
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(data))
	}

	var apiResp apiResponse
	if err := json.Unmarshal(data, &apiResp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	if apiResp.Error != nil {
		return nil, fmt.Errorf("API error %s: %s", apiResp.Error.Code, apiResp.Error.Message)
	}

	return apiResp.Result, nil
}

// Organization represents a ClickHouse Cloud organization.
type Organization struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Service represents a ClickHouse Cloud service.
type Service struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Provider  string    `json:"provider"`
	Region    string    `json:"region"`
	State     string    `json:"state"`
	Tier      string    `json:"tier"`
	Endpoints []Endpoint `json:"endpoints"`
	CreatedAt string    `json:"created_at"`
}

type Endpoint struct {
	Protocol string `json:"protocol"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
}

// ListOrganizations returns all organizations the API key has access to.
func (c *Client) ListOrganizations() ([]Organization, error) {
	data, err := c.do("GET", "/organizations", "")
	if err != nil {
		return nil, err
	}

	var orgs []Organization
	if err := json.Unmarshal(data, &orgs); err != nil {
		return nil, fmt.Errorf("parsing organizations: %w", err)
	}
	return orgs, nil
}

// ListServices returns all services in an organization.
func (c *Client) ListServices(orgID string) ([]Service, error) {
	data, err := c.do("GET", fmt.Sprintf("/organizations/%s/services", orgID), "")
	if err != nil {
		return nil, err
	}

	var services []Service
	if err := json.Unmarshal(data, &services); err != nil {
		return nil, fmt.Errorf("parsing services: %w", err)
	}
	return services, nil
}

// GetService returns details for a specific service.
func (c *Client) GetService(orgID, serviceID string) (*Service, error) {
	data, err := c.do("GET", fmt.Sprintf("/organizations/%s/services/%s", orgID, serviceID), "")
	if err != nil {
		return nil, err
	}

	var svc Service
	if err := json.Unmarshal(data, &svc); err != nil {
		return nil, fmt.Errorf("parsing service: %w", err)
	}
	return &svc, nil
}

// StartService starts a stopped/idle service.
func (c *Client) StartService(orgID, serviceID string) error {
	_, err := c.do("PATCH",
		fmt.Sprintf("/organizations/%s/services/%s/state", orgID, serviceID),
		`{"command":"start"}`)
	return err
}

// StopService stops a running service.
func (c *Client) StopService(orgID, serviceID string) error {
	_, err := c.do("PATCH",
		fmt.Sprintf("/organizations/%s/services/%s/state", orgID, serviceID),
		`{"command":"stop"}`)
	return err
}

// ServiceState constants.
const (
	StateRunning      = "running"
	StateStopped      = "stopped"
	StateIdle         = "idle"
	StateStarting     = "starting"
	StateStopping     = "stopping"
	StateProvisioning = "provisioning"
	StateDegraded     = "degraded"
	StateAwaking      = "awaking"
)

// IsRunning returns true if the service is in a running state.
func (s *Service) IsRunning() bool {
	return s.State == StateRunning
}

// IsStopped returns true if the service is stopped or idle.
func (s *Service) IsStopped() bool {
	return s.State == StateStopped || s.State == StateIdle
}

// NativeEndpoint returns the first native protocol endpoint, if any.
func (s *Service) NativeEndpoint() *Endpoint {
	for i, ep := range s.Endpoints {
		if ep.Protocol == "nativesecure" || ep.Protocol == "native" {
			return &s.Endpoints[i]
		}
	}
	return nil
}

// HTTPSEndpoint returns the first HTTPS endpoint, if any.
func (s *Service) HTTPSEndpoint() *Endpoint {
	for i, ep := range s.Endpoints {
		if ep.Protocol == "https" {
			return &s.Endpoints[i]
		}
	}
	return nil
}
