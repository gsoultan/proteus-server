package templates

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
)

var (
	// ErrNotFound is returned for unknown or someone else's templates.
	ErrNotFound = errors.New("template not found")
	// ErrNameTaken is returned when the owner already has a template with the name.
	ErrNameTaken = errors.New("a template with this name already exists")
	// ErrRevisionMismatch is returned by an update whose expected revision is stale.
	ErrRevisionMismatch = errors.New("template was changed by someone else; reload it and retry")
	// ErrLimit is returned when the owner has the maximum number of templates.
	ErrLimit = errors.New("template limit reached; delete templates you no longer need")
)

// Record is a template as a Store holds it.
type Record struct {
	ID          string
	Owner       string // API key id that created the template
	Name        string
	Description string
	Format      string // html or markdown
	Revision    int64
	SizeBytes   int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
	// Body is the serialized proteusv1.Template holding only content,
	// assets, options and sample data. List leaves it nil.
	Body []byte
}

// Store persists templates. Implementations must be safe for concurrent use.
type Store interface {
	// Create inserts r, filling in ID, Revision (1), CreatedAt and
	// UpdatedAt. ErrNameTaken when the owner has the name already.
	Create(ctx context.Context, r *Record) error
	// Get returns owner's template with its body; ErrNotFound otherwise.
	Get(ctx context.Context, owner, id string) (*Record, error)
	// List returns up to limit of owner's templates whose name sorts after
	// after, ordered by name, without bodies.
	List(ctx context.Context, owner, after string, limit int) ([]*Record, error)
	// Count returns how many templates owner has.
	Count(ctx context.Context, owner string) (int64, error)
	// Update replaces the name, description, format, size and body of
	// owner's template r.ID, bumps its revision and fills in r.Revision,
	// r.CreatedAt and r.UpdatedAt. A non-zero expected revision must match
	// (ErrRevisionMismatch otherwise).
	Update(ctx context.Context, r *Record, expected int64) error
	// Delete removes owner's template; ErrNotFound when there is none.
	Delete(ctx context.Context, owner, id string) error
}

// MemoryStore keeps templates in process memory; they are lost on restart.
// Use the Postgres store to keep them.
type MemoryStore struct {
	mu   sync.Mutex
	byID map[string]*Record
}

// NewMemoryStore returns an empty store.
func NewMemoryStore() *MemoryStore { return &MemoryStore{byID: map[string]*Record{}} }

func (s *MemoryStore) nameTakenLocked(owner, name, except string) bool {
	for _, r := range s.byID {
		if r.Owner == owner && r.Name == name && r.ID != except {
			return true
		}
	}
	return false
}

func clone(r *Record, withBody bool) *Record {
	c := *r
	c.Body = nil
	if withBody {
		c.Body = slices.Clone(r.Body)
	}
	return &c
}

// Create implements Store.
func (s *MemoryStore) Create(_ context.Context, r *Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.nameTakenLocked(r.Owner, r.Name, "") {
		return ErrNameTaken
	}
	now := time.Now().UTC()
	r.ID, r.Revision, r.CreatedAt, r.UpdatedAt = newUUID(), 1, now, now
	s.byID[r.ID] = clone(r, true)
	return nil
}

func (s *MemoryStore) ownedLocked(owner, id string) (*Record, error) {
	r, ok := s.byID[id]
	if !ok || r.Owner != owner {
		return nil, ErrNotFound
	}
	return r, nil
}

// Get implements Store.
func (s *MemoryStore) Get(_ context.Context, owner, id string) (*Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.ownedLocked(owner, id)
	if err != nil {
		return nil, err
	}
	return clone(r, true), nil
}

// List implements Store.
func (s *MemoryStore) List(_ context.Context, owner, after string, limit int) ([]*Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*Record
	for _, r := range s.byID {
		if r.Owner == owner && r.Name > after {
			out = append(out, clone(r, false))
		}
	}
	slices.SortFunc(out, func(a, b *Record) int { return strings.Compare(a.Name, b.Name) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// Count implements Store.
func (s *MemoryStore) Count(_ context.Context, owner string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var n int64
	for _, r := range s.byID {
		if r.Owner == owner {
			n++
		}
	}
	return n, nil
}

// Update implements Store.
func (s *MemoryStore) Update(_ context.Context, r *Record, expected int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, err := s.ownedLocked(r.Owner, r.ID)
	if err != nil {
		return err
	}
	if expected != 0 && cur.Revision != expected {
		return ErrRevisionMismatch
	}
	if s.nameTakenLocked(r.Owner, r.Name, r.ID) {
		return ErrNameTaken
	}
	r.Revision, r.CreatedAt, r.UpdatedAt = cur.Revision+1, cur.CreatedAt, time.Now().UTC()
	s.byID[r.ID] = clone(r, true)
	return nil
}

// Delete implements Store.
func (s *MemoryStore) Delete(_ context.Context, owner, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.ownedLocked(owner, id); err != nil {
		return err
	}
	delete(s.byID, id)
	return nil
}

func newUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

var _ Store = (*MemoryStore)(nil)
