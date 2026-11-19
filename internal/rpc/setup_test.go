// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

package rpc

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"

	proteusv1 "github.com/gsoultan/proteus-server/gen/proteus/v1"
	"github.com/gsoultan/proteus-server/gen/proteus/v1/proteusv1connect"
	"github.com/gsoultan/proteus-server/internal/auth"
	"github.com/gsoultan/proteus-server/internal/convert"
	"github.com/gsoultan/proteus-server/internal/jobs"
	"github.com/gsoultan/proteus-server/internal/metrics"
	"github.com/gsoultan/proteus-server/internal/safefetch"
	"github.com/gsoultan/proteus-server/internal/settings"
	"github.com/gsoultan/proteus-server/internal/templates"
)

const setupToken = "test-setup-token"

type setupEnv struct {
	url      string
	store    *settings.MemoryStore
	restarts atomic.Int32
}

// newSetupEnv is a fresh server with no keys at all, as on first run.
func newSetupEnv(t *testing.T, envVars map[string]string) *setupEnv {
	t.Helper()
	var a *API
	return newSetupEnvAPI(t, envVars, &a)
}

func newSetupEnvAPI(t *testing.T, envVars map[string]string, out **API) *setupEnv {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	m := metrics.New(1)
	svc := convert.NewService(convert.Settings{Workers: 1, Timeout: 20 * time.Second, Remote: safefetch.New(safefetch.Options{}), Logger: log, Metrics: m})
	jm := jobs.NewManager(svc, jobs.NewMemoryStore(0), jobs.Settings{Workers: 1, Logger: log, Observer: m})
	ts := templates.NewService(templates.NewMemoryStore(), templates.Settings{})
	e := &setupEnv{store: settings.NewMemoryStore()}
	api := New(Config{}, svc, jm, ts, auth.New(nil, 0, 0), m, log, nil)
	api.WithControl(&Control{
		Store: e.store, Env: func(k string) string { return envVars[k] }, DataDir: t.TempDir(),
		Storage: proteusv1.Storage_STORAGE_MEMORY, SetupToken: setupToken,
		OpenDatabase: func(context.Context, map[string]string, bool) (Database, error) {
			return nil, io.ErrUnexpectedEOF
		},
		FileStore: func() settings.Store { return e.store },
		Restart:   func() { e.restarts.Add(1) },
	})
	api.SetReady(true)
	*out = api
	srv := httptest.NewServer(api.Handler())
	t.Cleanup(func() {
		srv.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		jm.Shutdown(ctx)
	})
	e.url = srv.URL
	return e
}

func (e *setupEnv) client(key string) *connect.Client {
	return connect.NewClient(connecthttp.NewTransport(http.DefaultClient, e.url), withKey(key))
}

func (e *setupEnv) setup() proteusv1connect.SetupServiceClient {
	return proteusv1connect.NewSetupServiceClient(e.client(""))
}

func (e *setupEnv) admin(key string) proteusv1connect.AdminServiceClient {
	return proteusv1connect.NewAdminServiceClient(e.client(key))
}

func (e *setupEnv) conversion(key string) proteusv1connect.ConversionServiceClient {
	return proteusv1connect.NewConversionServiceClient(e.client(key))
}

func wantCode(t *testing.T, err error, code connect.Code) {
	t.Helper()
	if connect.CodeOf(err) != code {
		t.Fatalf("got %v, want %v", err, code)
	}
}

func TestSetupWizard(t *testing.T) {
	e := newSetupEnv(t, map[string]string{"PROTEUS_WORKERS": "3"})
	ctx := context.Background()
	setup := e.setup()

	st, err := setup.GetSetupStatus(ctx, &proteusv1.GetSetupStatusRequest{})
	if err != nil || !st.NeedsSetup || st.Storage != proteusv1.Storage_STORAGE_MEMORY {
		t.Fatalf("status = %v, %v", st, err)
	}
	// A fresh server is not open: every API call waits for setup.
	_, err = e.conversion("").ListFormats(ctx, &proteusv1.ListFormatsRequest{})
	wantCode(t, err, connect.CodeFailedPrecondition)
	if !strings.Contains(err.Error(), "/setup") {
		t.Fatalf("error does not point at /setup: %v", err)
	}
	_, err = e.admin("").GetSettings(ctx, &proteusv1.GetSettingsRequest{})
	wantCode(t, err, connect.CodeFailedPrecondition)

	_, err = setup.CheckSetupToken(ctx, &proteusv1.CheckSetupTokenRequest{SetupToken: "nope"})
	wantCode(t, err, connect.CodePermissionDenied)
	if _, err := setup.CheckSetupToken(ctx, &proteusv1.CheckSetupTokenRequest{SetupToken: " " + setupToken + " "}); err != nil {
		t.Fatal(err)
	}
	_, err = setup.TestDatabase(ctx, &proteusv1.TestDatabaseRequest{SetupToken: "nope", Database: &proteusv1.Database{Url: "postgres://x"}})
	wantCode(t, err, connect.CodePermissionDenied)
	res, err := setup.TestDatabase(ctx, &proteusv1.TestDatabaseRequest{SetupToken: setupToken, Database: &proteusv1.Database{Url: "postgres://x"}})
	if err != nil || res.Ok || res.Error == "" {
		t.Fatalf("TestDatabase = %v, %v", res, err)
	}

	for name, s := range map[string]map[string]string{
		"bad value": {"PROTEUS_TIMEOUT": "soon"},
		"bootstrap": {"PROTEUS_ADDR": ":1"},
		"unknown":   {"PROTEUS_NOPE": "1"},
	} {
		_, err := setup.CompleteSetup(ctx, &proteusv1.CompleteSetupRequest{SetupToken: setupToken, Settings: s})
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: %v", name, err)
		}
	}
	_, err = setup.CompleteSetup(ctx, &proteusv1.CompleteSetupRequest{SetupToken: setupToken, AdminKeyName: "<script>"})
	wantCode(t, err, connect.CodeInvalidArgument)
	if e.restarts.Load() != 0 {
		t.Fatal("a failed setup restarted the server")
	}

	done, err := setup.CompleteSetup(ctx, &proteusv1.CompleteSetupRequest{
		SetupToken: setupToken, Storage: proteusv1.Storage_STORAGE_MEMORY, AdminKeyName: "Ops",
		Settings: map[string]string{"PROTEUS_TIMEOUT": "5s", "PROTEUS_WEBHOOK_SECRET": strings.Repeat("s", 32)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if done.Secret == "" || done.AdminKey.Role != proteusv1.ApiKey_ROLE_ADMIN || done.AdminKey.Name != "Ops" || !strings.HasPrefix(done.Secret, done.AdminKey.Prefix) {
		t.Fatalf("CompleteSetup = %v", done)
	}
	if e.restarts.Load() != 1 {
		t.Fatalf("restarts = %d", e.restarts.Load())
	}

	// The wizard is closed for good.
	if st, _ = setup.GetSetupStatus(ctx, &proteusv1.GetSetupStatusRequest{}); st.NeedsSetup {
		t.Fatal("still needs setup")
	}
	_, err = setup.CompleteSetup(ctx, &proteusv1.CompleteSetupRequest{SetupToken: setupToken})
	wantCode(t, err, connect.CodeFailedPrecondition)
	_, err = setup.CheckSetupToken(ctx, &proteusv1.CheckSetupTokenRequest{SetupToken: setupToken})
	wantCode(t, err, connect.CodeFailedPrecondition)

	// The server restarts in production, which loads the keys; do that here.
	keys, _ := e.store.ListKeys(ctx)
	if len(keys) != 1 {
		t.Fatalf("keys = %v", keys)
	}
}

// setUp runs the wizard and returns an admin key, with the keys loaded as a
// restart would.
func setUp(t *testing.T, e *setupEnv, api func() *API) string {
	t.Helper()
	done, err := e.setup().CompleteSetup(context.Background(), &proteusv1.CompleteSetupRequest{SetupToken: setupToken, Settings: map[string]string{"PROTEUS_TIMEOUT": "5s"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := api().ReloadKeys(context.Background()); err != nil {
		t.Fatal(err)
	}
	return done.Secret
}

func TestAdminSettingsAndKeys(t *testing.T) {
	var a *API
	e := newSetupEnvAPI(t, map[string]string{"PROTEUS_WORKERS": "3"}, &a)
	ctx := context.Background()
	adminKey := setUp(t, e, func() *API { return a })

	if _, err := e.conversion("").ListFormats(ctx, &proteusv1.ListFormatsRequest{}); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("no key after setup: %v", err)
	}
	if _, err := e.conversion(adminKey).ListFormats(ctx, &proteusv1.ListFormatsRequest{}); err != nil {
		t.Fatal(err)
	}

	adm := e.admin(adminKey)
	got, err := adm.GetSettings(ctx, &proteusv1.GetSettingsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]*proteusv1.Setting{}
	for _, s := range got.Settings {
		by[s.Name] = s
	}
	if s := by["PROTEUS_TIMEOUT"]; s.Value != "5s" || s.Source != proteusv1.Setting_SOURCE_SAVED || !s.Editable {
		t.Fatalf("timeout = %v", s)
	}
	if s := by["PROTEUS_WORKERS"]; s.Value != "3" || s.Source != proteusv1.Setting_SOURCE_ENVIRONMENT {
		t.Fatalf("workers = %v", s)
	}
	if s := by["PROTEUS_ADDR"]; s.Editable || s.Source != proteusv1.Setting_SOURCE_DEFAULT {
		t.Fatalf("addr = %v", s)
	}

	_, err = adm.UpdateSettings(ctx, &proteusv1.UpdateSettingsRequest{Revision: got.Revision + 5, Values: map[string]string{"PROTEUS_TIMEOUT": "6s"}})
	wantCode(t, err, connect.CodeAborted)
	_, err = adm.UpdateSettings(ctx, &proteusv1.UpdateSettingsRequest{Revision: got.Revision, Values: map[string]string{"PROTEUS_TIMEOUT": "later"}})
	wantCode(t, err, connect.CodeInvalidArgument)
	_, err = adm.UpdateSettings(ctx, &proteusv1.UpdateSettingsRequest{Revision: got.Revision, Values: map[string]string{"PROTEUS_LOG_LEVEL": "debug"}})
	wantCode(t, err, connect.CodeInvalidArgument)
	restarts := e.restarts.Load()
	up, err := adm.UpdateSettings(ctx, &proteusv1.UpdateSettingsRequest{Revision: got.Revision, Values: map[string]string{
		"PROTEUS_TIMEOUT": "", "PROTEUS_WEBHOOK_SECRET": strings.Repeat("w", 40), "PROTEUS_ALLOWED_HOSTS": "a.com,.b.com",
	}})
	if err != nil || !up.Restarting || up.Revision != got.Revision+1 || e.restarts.Load() != restarts+1 {
		t.Fatalf("UpdateSettings = %v, %v", up, err)
	}
	got, _ = adm.GetSettings(ctx, &proteusv1.GetSettingsRequest{})
	for _, s := range got.Settings {
		by[s.Name] = s
	}
	if s := by["PROTEUS_TIMEOUT"]; s.Source != proteusv1.Setting_SOURCE_DEFAULT || s.Value != "30s" {
		t.Fatalf("cleared timeout = %v", s)
	}
	if s := by["PROTEUS_WEBHOOK_SECRET"]; s.Value != "" || !s.IsSet {
		t.Fatalf("secret leaked or lost: %v", s)
	}
	// Saving nothing new does not restart.
	if up, err = adm.UpdateSettings(ctx, &proteusv1.UpdateSettingsRequest{Revision: got.Revision}); err != nil || up.Restarting {
		t.Fatalf("no-op save = %v, %v", up, err)
	}

	// Keys: a client key works for the API but not for the admin pages.
	ck, err := adm.CreateApiKey(ctx, &proteusv1.CreateApiKeyRequest{Name: "Billing app"})
	if err != nil || ck.Key.Role != proteusv1.ApiKey_ROLE_CLIENT || ck.Secret == "" {
		t.Fatalf("CreateApiKey = %v, %v", ck, err)
	}
	if _, err := e.conversion(ck.Secret).ListFormats(ctx, &proteusv1.ListFormatsRequest{}); err != nil {
		t.Fatalf("new key not live: %v", err)
	}
	_, err = e.admin(ck.Secret).GetSettings(ctx, &proteusv1.GetSettingsRequest{})
	wantCode(t, err, connect.CodePermissionDenied)

	list, err := adm.ListApiKeys(ctx, &proteusv1.ListApiKeysRequest{})
	if err != nil || len(list.Keys) != 2 {
		t.Fatalf("ListApiKeys = %v, %v", list, err)
	}
	self := list.CallerId
	_, err = adm.DeleteApiKey(ctx, &proteusv1.DeleteApiKeyRequest{Id: self})
	wantCode(t, err, connect.CodeFailedPrecondition)
	_, err = adm.DeleteApiKey(ctx, &proteusv1.DeleteApiKeyRequest{Id: "k_missing"})
	wantCode(t, err, connect.CodeNotFound)

	if _, err := adm.DeleteApiKey(ctx, &proteusv1.DeleteApiKeyRequest{Id: ck.Key.Id}); err != nil {
		t.Fatal(err)
	}
	_, err = e.conversion(ck.Secret).ListFormats(ctx, &proteusv1.ListFormatsRequest{})
	wantCode(t, err, connect.CodeUnauthenticated)

	// A second admin can revoke the first; the last admin cannot go.
	ak, err := adm.CreateApiKey(ctx, &proteusv1.CreateApiKeyRequest{Name: "Second", Role: proteusv1.ApiKey_ROLE_ADMIN})
	if err != nil {
		t.Fatal(err)
	}
	adm2 := e.admin(ak.Secret)
	if _, err := adm2.DeleteApiKey(ctx, &proteusv1.DeleteApiKeyRequest{Id: self}); err != nil {
		t.Fatal(err)
	}
	cl, err := adm2.CreateApiKey(ctx, &proteusv1.CreateApiKeyRequest{Name: "Other"})
	if err != nil {
		t.Fatal(err)
	}
	_ = cl
	_, err = adm2.DeleteApiKey(ctx, &proteusv1.DeleteApiKeyRequest{Id: ak.Key.Id})
	wantCode(t, err, connect.CodeFailedPrecondition)
}
