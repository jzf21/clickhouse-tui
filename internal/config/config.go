package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Connection struct {
	Name     string `json:"name"`
	Host     string `json:"host"`
	Port     string `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database"`
}

type CloudCredentials struct {
	APIKey          string   `json:"api_key"`
	APISecret       string   `json:"api_secret"`
	OrgID           string   `json:"org_id,omitempty"`
	AllowedServices []string `json:"allowed_services,omitempty"` // service IDs visible in dashboard
}

type Store struct {
	Connections []Connection     `json:"connections"`
	Cloud       CloudCredentials `json:"cloud,omitempty"`
	path        string
}

func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".clickhouse-tui")
	return dir, os.MkdirAll(dir, 0755)
}

func Load() (*Store, error) {
	dir, err := configDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "connections.json")
	store := &Store{path: path}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return store, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, store); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *Store) Save() error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0644)
}

func (s *Store) Add(c Connection) error {
	for _, existing := range s.Connections {
		if existing.Name == c.Name {
			return fmt.Errorf("connection %q already exists", c.Name)
		}
	}
	s.Connections = append(s.Connections, c)
	return s.Save()
}

func (s *Store) Delete(name string) error {
	for i, c := range s.Connections {
		if c.Name == name {
			s.Connections = append(s.Connections[:i], s.Connections[i+1:]...)
			return s.Save()
		}
	}
	return fmt.Errorf("connection %q not found", name)
}

func (s *Store) SetCloud(creds CloudCredentials) error {
	s.Cloud = creds
	return s.Save()
}

func (s *Store) HasCloud() bool {
	return s.Cloud.APIKey != "" && s.Cloud.APISecret != ""
}

// IsServiceAllowed returns true if the service ID is in the allowlist,
// or if the allowlist is empty (all services allowed).
func (s *Store) IsServiceAllowed(serviceID string) bool {
	if len(s.Cloud.AllowedServices) == 0 {
		return true
	}
	for _, id := range s.Cloud.AllowedServices {
		if id == serviceID {
			return true
		}
	}
	return false
}

// ToggleServiceAllowed adds or removes a service ID from the allowlist.
func (s *Store) ToggleServiceAllowed(serviceID string) error {
	for i, id := range s.Cloud.AllowedServices {
		if id == serviceID {
			s.Cloud.AllowedServices = append(s.Cloud.AllowedServices[:i], s.Cloud.AllowedServices[i+1:]...)
			return s.Save()
		}
	}
	s.Cloud.AllowedServices = append(s.Cloud.AllowedServices, serviceID)
	return s.Save()
}

// ClearAllowedServices removes all entries from the allowlist (shows all services).
func (s *Store) ClearAllowedServices() error {
	s.Cloud.AllowedServices = nil
	return s.Save()
}
