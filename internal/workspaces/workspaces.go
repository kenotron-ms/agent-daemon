package workspaces

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	bolt "go.etcd.io/bbolt"
)

var bucketProjects = []byte("projects")

// Project is a codebase on disk managed by Loom.
type Project struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Path           string `json:"path"`      // absolute path on disk
	Workspace      string `json:"workspace"` // grouping label (default: "Default")
	CreatedAt      int64  `json:"createdAt"`
	LastActivityAt int64  `json:"lastActivityAt"`
}

// Service is the workspace CRUD layer backed by bbolt.
type Service struct {
	db *bolt.DB
}

// New creates a Service and initialises the required bbolt buckets.
func New(db *bolt.DB) (*Service, error) {
	err := db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(bucketProjects); err != nil {
			return fmt.Errorf("create bucket %q: %w", bucketProjects, err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("initialise workspaces buckets: %w", err)
	}
	return &Service{db: db}, nil
}

// ── Projects ──────────────────────────────────────────────────────────────────

func (s *Service) CreateProject(_ context.Context, name, path string) (*Project, error) {
	p := &Project{
		ID:             uuid.New().String(),
		Name:           name,
		Path:           path,
		CreatedAt:      time.Now().Unix(),
		LastActivityAt: time.Now().Unix(),
	}
	return p, s.db.Update(func(tx *bolt.Tx) error {
		data, err := json.Marshal(p)
		if err != nil {
			return err
		}
		return tx.Bucket(bucketProjects).Put([]byte(p.ID), data)
	})
}

func (s *Service) GetProject(_ context.Context, id string) (*Project, error) {
	var p Project
	err := s.db.View(func(tx *bolt.Tx) error {
		data := tx.Bucket(bucketProjects).Get([]byte(id))
		if data == nil {
			return fmt.Errorf("project %s not found", id)
		}
		return json.Unmarshal(data, &p)
	})
	if err != nil {
		return nil, err
	}
	if p.Workspace == "" {
		p.Workspace = "Default"
	}
	return &p, nil
}

func (s *Service) ListProjects(_ context.Context) ([]*Project, error) {
	var projects []*Project
	err := s.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketProjects).ForEach(func(_, v []byte) error {
			var p Project
			if err := json.Unmarshal(v, &p); err != nil {
				return err
			}
			if p.Workspace == "" {
				p.Workspace = "Default"
			}
			projects = append(projects, &p)
			return nil
		})
	})
	if projects == nil {
		projects = []*Project{}
	}
	return projects, err
}

func (s *Service) UpdateProject(_ context.Context, id, name, workspace string) (*Project, error) {
	var p Project
	return &p, s.db.Update(func(tx *bolt.Tx) error {
		data := tx.Bucket(bucketProjects).Get([]byte(id))
		if data == nil {
			return fmt.Errorf("project %s not found", id)
		}
		if err := json.Unmarshal(data, &p); err != nil {
			return err
		}
		p.Name = name
		if workspace != "" {
			p.Workspace = workspace
		}
		if p.Workspace == "" {
			p.Workspace = "Default"
		}
		updated, err := json.Marshal(p)
		if err != nil {
			return err
		}
		return tx.Bucket(bucketProjects).Put([]byte(id), updated)
	})
}

func (s *Service) DeleteProject(_ context.Context, id string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketProjects).Delete([]byte(id))
	})
}

func (s *Service) TouchProject(_ context.Context, id string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		data := tx.Bucket(bucketProjects).Get([]byte(id))
		if data == nil {
			return fmt.Errorf("project %s not found", id)
		}
		var p Project
		if err := json.Unmarshal(data, &p); err != nil {
			return err
		}
		p.LastActivityAt = time.Now().Unix()
		updated, err := json.Marshal(p)
		if err != nil {
			return err
		}
		return tx.Bucket(bucketProjects).Put([]byte(id), updated)
	})
}
