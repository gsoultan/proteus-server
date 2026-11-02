// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

package templates

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"google.golang.org/protobuf/types/known/structpb"

	proteusv1 "github.com/gsoultan/proteus-server/gen/proteus/v1"
)

// Field paths are dotted identifiers, as Go templates address them.
var (
	identRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	pathRe  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)*$`)
)

const (
	maxFields     = 200
	maxFieldDepth = 4
	maxChoices    = 100
)

// validPath reports whether p is a dotted field path.
func validPath(p string) bool { return len(p) <= 200 && pathRe.MatchString(p) }

// ValidateFields checks a template's field list: paths, types, nesting and
// duplicates.
func ValidateFields(fields []*proteusv1.FieldSpec) error {
	n := 0
	return validateFields(fields, "", 0, &n)
}

func validateFields(fields []*proteusv1.FieldSpec, prefix string, depth int, n *int) error {
	seen := map[string]bool{}
	for _, f := range fields {
		*n++
		if *n > maxFields {
			return bad(fmt.Sprintf("more than %d fields", maxFields))
		}
		p := f.GetPath()
		full := prefix + p
		switch {
		case p == "":
			return bad("a field has no path")
		case depth == 0 && !validPath(p):
			return bad(fmt.Sprintf("field %q: a path is dotted names such as client.name", full))
		case depth > 0 && !identRe.MatchString(p):
			return bad(fmt.Sprintf("field %q: an item field is a single name", full))
		case seen[p]:
			return bad(fmt.Sprintf("field %q is listed twice", full))
		case utf8.RuneCountInString(f.GetLabel()) > 200 || utf8.RuneCountInString(f.GetDescription()) > 1000:
			return bad(fmt.Sprintf("field %q: label or description too long", full))
		}
		seen[p] = true
		if _, ok := proteusv1.FieldType_name[int32(f.GetType())]; !ok {
			return bad(fmt.Sprintf("field %q: unknown type %v", full, f.GetType()))
		}
		if len(f.GetChoices()) > maxChoices {
			return bad(fmt.Sprintf("field %q: more than %d choices", full, maxChoices))
		}
		if f.GetType() == proteusv1.FieldType_FIELD_TYPE_CHOICE && len(f.GetChoices()) == 0 {
			return bad(fmt.Sprintf("field %q: a choice field needs choices", full))
		}
		if d := f.GetDefaultValue(); d != nil {
			c := &dataChecker{}
			c.value(f, d, full)
			if len(c.problems) > 0 {
				return bad(fmt.Sprintf("field %q: default value %s", full, c.problems[0].GetMessage()))
			}
		}
		nested := f.GetType() == proteusv1.FieldType_FIELD_TYPE_LIST || f.GetType() == proteusv1.FieldType_FIELD_TYPE_OBJECT
		if len(f.GetItemFields()) > 0 && !nested {
			return bad(fmt.Sprintf("field %q: only list and object fields have item fields", full))
		}
		if nested {
			if depth+1 > maxFieldDepth {
				return bad(fmt.Sprintf("field %q: fields nest more than %d deep", full, maxFieldDepth))
			}
			if err := validateFields(f.GetItemFields(), full+".", depth+1, n); err != nil {
				return err
			}
		}
	}
	return nil
}

// DataError lists what is wrong with the data sent for a template. It is
// a BadRequestError for the error mapping and carries the problems for the
// TemplateDataError detail.
type DataError struct {
	Problems []*proteusv1.DataProblem
}

func (e *DataError) Error() string {
	var b strings.Builder
	b.WriteString("data does not match the template's fields: ")
	for i, p := range e.Problems {
		if i > 0 {
			b.WriteString("; ")
		}
		if i == 5 {
			fmt.Fprintf(&b, "and %d more", len(e.Problems)-i)
			break
		}
		b.WriteString(p.GetPath() + ": " + p.GetMessage())
	}
	return b.String()
}

// Detail returns the problems as the proto error detail.
func (e *DataError) Detail() *proteusv1.TemplateDataError {
	return &proteusv1.TemplateDataError{Problems: e.Problems}
}

const maxProblems = 50

// CheckData validates data against fields and returns every problem found
// as a *DataError, or nil. Fields the data has but the template does not
// list are allowed. Defaults are not applied here; see ApplyDefaults.
func CheckData(fields []*proteusv1.FieldSpec, data *structpb.Struct) error {
	if len(fields) == 0 {
		return nil
	}
	c := &dataChecker{}
	c.object(fields, data.GetFields(), "")
	if len(c.problems) == 0 {
		return nil
	}
	return &DataError{Problems: c.problems}
}

type dataChecker struct {
	problems []*proteusv1.DataProblem
}

func (c *dataChecker) add(path, msg string) {
	if len(c.problems) < maxProblems {
		c.problems = append(c.problems, &proteusv1.DataProblem{Path: path, Message: msg})
	}
}

// lookup follows a dotted path through nested structs.
func lookup(fields map[string]*structpb.Value, path string) (*structpb.Value, bool) {
	parts := strings.Split(path, ".")
	for i, p := range parts {
		v, ok := fields[p]
		if !ok {
			return nil, false
		}
		if i == len(parts)-1 {
			return v, true
		}
		st := v.GetStructValue()
		if st == nil {
			return nil, false
		}
		fields = st.GetFields()
	}
	return nil, false
}

func (c *dataChecker) object(fields []*proteusv1.FieldSpec, data map[string]*structpb.Value, prefix string) {
	for _, f := range fields {
		full := prefix + f.GetPath()
		v, ok := lookup(data, f.GetPath())
		if !ok || isEmptyValue(v) {
			if f.GetRequired() && f.GetDefaultValue() == nil {
				c.add(full, "is required")
			}
			continue
		}
		c.value(f, v, full)
	}
}

// isEmptyValue reports whether v counts as "not given": nil, null, blank
// text or an empty list.
func isEmptyValue(v *structpb.Value) bool {
	if v == nil {
		return true
	}
	switch k := v.GetKind().(type) {
	case nil, *structpb.Value_NullValue:
		return true
	case *structpb.Value_StringValue:
		return strings.TrimSpace(k.StringValue) == ""
	case *structpb.Value_ListValue:
		return len(k.ListValue.GetValues()) == 0
	}
	return false
}

func (c *dataChecker) value(f *proteusv1.FieldSpec, v *structpb.Value, path string) {
	switch f.GetType() {
	case proteusv1.FieldType_FIELD_TYPE_UNSPECIFIED, proteusv1.FieldType_FIELD_TYPE_TEXT, proteusv1.FieldType_FIELD_TYPE_IMAGE:
		switch v.GetKind().(type) {
		case *structpb.Value_StringValue, *structpb.Value_NumberValue, *structpb.Value_BoolValue:
		default:
			c.add(path, "must be text")
		}
	case proteusv1.FieldType_FIELD_TYPE_NUMBER, proteusv1.FieldType_FIELD_TYPE_MONEY:
		switch k := v.GetKind().(type) {
		case *structpb.Value_NumberValue:
		case *structpb.Value_StringValue:
			if _, err := strconv.ParseFloat(strings.TrimSpace(k.StringValue), 64); err != nil {
				c.add(path, "must be a number")
			}
		default:
			c.add(path, "must be a number")
		}
	case proteusv1.FieldType_FIELD_TYPE_DATE:
		s := v.GetStringValue()
		ok := false
		for _, layout := range []string{time.RFC3339Nano, time.RFC3339, time.DateOnly} {
			if _, err := time.Parse(layout, s); err == nil {
				ok = true
				break
			}
		}
		if !ok {
			c.add(path, "must be a date such as 2026-11-02 or an RFC 3339 time")
		}
	case proteusv1.FieldType_FIELD_TYPE_BOOL:
		if _, ok := v.GetKind().(*structpb.Value_BoolValue); !ok {
			c.add(path, "must be true or false")
		}
	case proteusv1.FieldType_FIELD_TYPE_CHOICE:
		s := v.GetStringValue()
		found := false
		for _, ch := range f.GetChoices() {
			if ch == s {
				found = true
				break
			}
		}
		if !found {
			c.add(path, "must be one of "+strings.Join(f.GetChoices(), ", "))
		}
	case proteusv1.FieldType_FIELD_TYPE_LIST:
		list := v.GetListValue()
		if list == nil {
			c.add(path, "must be a list")
			return
		}
		for i, item := range list.GetValues() {
			st := item.GetStructValue()
			if st == nil {
				if len(f.GetItemFields()) > 0 {
					c.add(fmt.Sprintf("%s[%d]", path, i), "must be an object")
				}
				continue
			}
			c.object(f.GetItemFields(), st.GetFields(), fmt.Sprintf("%s[%d].", path, i))
		}
	case proteusv1.FieldType_FIELD_TYPE_OBJECT:
		st := v.GetStructValue()
		if st == nil {
			c.add(path, "must be an object")
			return
		}
		c.object(f.GetItemFields(), st.GetFields(), path+".")
	}
}

// ApplyDefaults returns data with the default of every listed field that
// has none, so a template can rely on them. data is not modified.
func ApplyDefaults(fields []*proteusv1.FieldSpec, data *structpb.Struct) *structpb.Struct {
	if len(fields) == 0 {
		return data
	}
	out := &structpb.Struct{Fields: map[string]*structpb.Value{}}
	for k, v := range data.GetFields() {
		out.Fields[k] = v
	}
	applyDefaults(fields, out)
	return out
}

func applyDefaults(fields []*proteusv1.FieldSpec, st *structpb.Struct) {
	for _, f := range fields {
		if f.GetDefaultValue() == nil || strings.Contains(f.GetPath(), ".") {
			continue
		}
		if v, ok := st.Fields[f.GetPath()]; !ok || isEmptyValue(v) {
			if st.Fields == nil {
				st.Fields = map[string]*structpb.Value{}
			}
			st.Fields[f.GetPath()] = f.GetDefaultValue()
		}
	}
}
