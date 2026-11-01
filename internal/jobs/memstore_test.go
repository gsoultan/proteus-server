// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

package jobs_test

import (
	"testing"

	"github.com/gsoultan/proteus-server/internal/jobs"
	"github.com/gsoultan/proteus-server/internal/jobs/storetest"
)

func TestMemoryStore(t *testing.T) { storetest.Run(t, jobs.NewMemoryStore(0)) }
