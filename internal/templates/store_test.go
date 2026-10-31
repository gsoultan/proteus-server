package templates_test

import (
	"testing"

	"github.com/gsoultan/proteus-server/internal/templates"
	"github.com/gsoultan/proteus-server/internal/templates/storetest"
)

func TestMemoryStore(t *testing.T) { storetest.Run(t, templates.NewMemoryStore()) }
