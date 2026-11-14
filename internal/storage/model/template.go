package model

import "github.com/gsoultan/storm"

// Template is a stored document template. Body is the serialized
// templates.Body (content, assets, options and sample data); listing reads
// the Meta projection so it never pulls the body.
type Template struct {
	storm.Model // uuid id + created_at/updated_at

	Owner       string // API key id that created the template
	Name        string
	Description string
	Format      string // html or markdown
	Revision    int64
	SizeBytes   int64
	Body        []byte
}

func (t *Template) Schema(s *storm.Table) {
	s.Col(&t.Owner).Size(128)
	s.Col(&t.Name).Size(200)
	s.Col(&t.Description).Size(2000)
	s.Col(&t.Format).Size(16)
	s.Col(&t.Revision).Default("1")
	s.Col(&t.SizeBytes).Default("0")
	// Names are unique per key; the index also serves the listing order.
	s.Unique(&t.Owner, &t.Name)
}

// Projections: Meta is every column but the body.
func (t *Template) Projections(p *storm.Projections) {
	p.Named("Meta", &t.ID, &t.Owner, &t.Name, &t.Description, &t.Format, &t.Revision,
		&t.SizeBytes, &t.CreatedAt, &t.UpdatedAt)
}
