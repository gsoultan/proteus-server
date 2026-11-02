// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

package templates

import (
	"embed"
	"fmt"
	"path"
	"slices"
	"strings"
	"sync"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	proteusv1 "github.com/gsoultan/proteus-server/gen/proteus/v1"
)

// Starters are complete visual templates shipped with the server, one JSON
// file (protojson of proteusv1.Template) per document kind. They use only
// the public layout model, so each one is also a fixture: the tests
// compile and render every starter with its sample data.
//
//go:embed starters/*.json
var starterFS embed.FS

// starterOrder is the order ListStarterTemplates returns them in.
var starterOrder = []string{"invoice", "quotation", "receipt", "purchase_order", "delivery_note", "letter", "certificate", "report", "blank"}

var (
	startersOnce sync.Once
	startersByID map[string]*proteusv1.Template
	startersErr  error
)

func loadStarters() {
	startersByID = map[string]*proteusv1.Template{}
	entries, err := starterFS.ReadDir("starters")
	if err != nil {
		startersErr = err
		return
	}
	for _, e := range entries {
		data, err := starterFS.ReadFile(path.Join("starters", e.Name()))
		if err != nil {
			startersErr = err
			return
		}
		t := &proteusv1.Template{}
		if err := protojson.Unmarshal(data, t); err != nil {
			startersErr = fmt.Errorf("starter %s: %w", e.Name(), err)
			return
		}
		id := strings.TrimSuffix(e.Name(), ".json")
		t.StarterId = id
		startersByID[id] = t
	}
}

// Starter returns a copy of the starter with the given id.
func Starter(id string) (*proteusv1.Template, bool) {
	startersOnce.Do(loadStarters)
	t, ok := startersByID[id]
	if !ok {
		return nil, false
	}
	return proto.CloneOf(t), true
}

// Starters returns copies of every starter, optionally only those in
// category, in display order.
func Starters(category string) ([]*proteusv1.Template, error) {
	startersOnce.Do(loadStarters)
	if startersErr != nil {
		return nil, startersErr
	}
	ids := slices.Clone(starterOrder)
	for id := range startersByID {
		if !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	var out []*proteusv1.Template
	for _, id := range ids {
		t, ok := startersByID[id]
		if !ok || (category != "" && t.GetCategory() != category) {
			continue
		}
		out = append(out, proto.CloneOf(t))
	}
	return out, nil
}
