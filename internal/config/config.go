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
	APIKey    string `json:"api_key"`
	APISecret string `json:"api_secret"`
	OrgID     string `json:"org_id,omitempty"`
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
