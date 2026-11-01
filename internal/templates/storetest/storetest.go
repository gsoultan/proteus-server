// Package storetest is a conformance suite every templates.Store must pass.
package storetest

import (
	"context"
	"errors"
	"testing"

	"github.com/gsoultan/proteus-server/internal/templates"
)

// Run exercises s. The store must be empty.
func Run(t *testing.T, s templates.Store) {
	ctx := context.Background()

	a := &templates.Record{Owner: "alice", Name: "invoice", Description: "d", Format: "html", SizeBytes: 4, Body: []byte("body")}
	t.Run("Create", func(t *testing.T) {
		if err := s.Create(ctx, a); err != nil {
			t.Fatal(err)
		}
		if a.ID == "" || a.Revision != 1 || a.CreatedAt.IsZero() || a.UpdatedAt.IsZero() {
			t.Fatalf("created %+v", a)
		}
		dup := &templates.Record{Owner: "alice", Name: "invoice", Format: "html", Body: []byte("x")}
		if err := s.Create(ctx, dup); !errors.Is(err, templates.ErrNameTaken) {
			t.Fatalf("duplicate name = %v", err)
		}
		// Names are per owner.
		other := &templates.Record{Owner: "bob", Name: "invoice", Format: "markdown", Body: []byte("x")}
		if err := s.Create(ctx, other); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("GetAndScoping", func(t *testing.T) {
		got, err := s.Get(ctx, "alice", a.ID)
		if err != nil || string(got.Body) != "body" || got.Name != "invoice" || got.Format != "html" ||
			got.Description != "d" || got.SizeBytes != 4 || got.Revision != 1 {
			t.Fatalf("Get = %+v, %v", got, err)
		}
		if _, err := s.Get(ctx, "bob", a.ID); !errors.Is(err, templates.ErrNotFound) {
			t.Fatalf("bob Get = %v", err)
		}
		if _, err := s.Get(ctx, "alice", "not-a-uuid"); !errors.Is(err, templates.ErrNotFound) {
			t.Fatalf("bad id Get = %v", err)
		}
	})

	t.Run("ListAndCount", func(t *testing.T) {
		for _, n := range []string{"b", "c", "a"} {
			if err := s.Create(ctx, &templates.Record{Owner: "alice", Name: n, Format: "html", Body: []byte(n)}); err != nil {
				t.Fatal(err)
			}
		}
		if n, err := s.Count(ctx, "alice"); err != nil || n != 4 {
			t.Fatalf("Count = %d, %v", n, err)
		}
		page, err := s.List(ctx, "alice", "", 2)
		if err != nil || len(page) != 2 || page[0].Name != "a" || page[1].Name != "b" || page[0].Body != nil {
			t.Fatalf("List page 1 = %+v, %v", page, err)
		}
		page, err = s.List(ctx, "alice", "b", 10)
		if err != nil || len(page) != 2 || page[0].Name != "c" || page[1].Name != "invoice" {
			t.Fatalf("List page 2 = %+v, %v", page, err)
		}
		if page, err := s.List(ctx, "carol", "", 10); err != nil || len(page) != 0 {
			t.Fatalf("carol List = %+v, %v", page, err)
		}
	})

	t.Run("Update", func(t *testing.T) {
		u := &templates.Record{ID: a.ID, Owner: "alice", Name: "invoice-v2", Format: "markdown", SizeBytes: 2, Body: []byte("v2")}
		if err := s.Update(ctx, u, 1); err != nil {
			t.Fatal(err)
		}
		if u.Revision != 2 || !u.CreatedAt.Equal(a.CreatedAt) {
			t.Fatalf("updated %+v", u)
		}
		got, err := s.Get(ctx, "alice", a.ID)
		if err != nil || got.Name != "invoice-v2" || string(got.Body) != "v2" || got.Format != "markdown" || got.Revision != 2 {
			t.Fatalf("after update %+v, %v", got, err)
		}
		if err := s.Update(ctx, u, 1); !errors.Is(err, templates.ErrRevisionMismatch) {
			t.Fatalf("stale update = %v", err)
		}
		// Revision 0 means "whatever is stored".
		if err := s.Update(ctx, u, 0); err != nil || u.Revision != 3 {
			t.Fatalf("unconditional update = %v (rev %d)", err, u.Revision)
		}
		taken := &templates.Record{ID: a.ID, Owner: "alice", Name: "a", Format: "html", Body: []byte("x")}
		if err := s.Update(ctx, taken, 0); !errors.Is(err, templates.ErrNameTaken) {
			t.Fatalf("rename onto existing = %v", err)
		}
		bob := &templates.Record{ID: a.ID, Owner: "bob", Name: "z", Format: "html", Body: []byte("x")}
		if err := s.Update(ctx, bob, 0); !errors.Is(err, templates.ErrNotFound) {
			t.Fatalf("bob update = %v", err)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		if err := s.Delete(ctx, "bob", a.ID); !errors.Is(err, templates.ErrNotFound) {
			t.Fatalf("bob Delete = %v", err)
		}
		if err := s.Delete(ctx, "alice", a.ID); err != nil {
			t.Fatal(err)
		}
		if err := s.Delete(ctx, "alice", a.ID); !errors.Is(err, templates.ErrNotFound) {
			t.Fatalf("second Delete = %v", err)
		}
		if _, err := s.Get(ctx, "alice", a.ID); !errors.Is(err, templates.ErrNotFound) {
			t.Fatalf("Get after Delete = %v", err)
		}
	})
}
