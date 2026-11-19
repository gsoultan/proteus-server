// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

package rpc

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base32"
	"errors"
	"maps"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"connectrpc.com/connect/v2"
	"golang.org/x/time/rate"
	"google.golang.org/protobuf/types/known/timestamppb"

	proteusv1 "github.com/gsoultan/proteus-server/gen/proteus/v1"
	"github.com/gsoultan/proteus-server/gen/proteus/v1/proteusv1connect"
	"github.com/gsoultan/proteus-server/internal/auth"
	"github.com/gsoultan/proteus-server/internal/config"
	"github.com/gsoultan/proteus-server/internal/settings"
)

// Database is an open PostgreSQL connection the setup wizard tests or sets
// up.
type Database interface {
	Settings() settings.Store
	ServerVersion(ctx context.Context) (string, error)
	Close()
}

// Control is what the setup wizard and the admin pages need from the
// process: where settings live, how they were layered, and how to apply a
// change (an in-place restart of every service).
type Control struct {
	Store          settings.Store      // settings and UI keys in use now
	Env            func(string) string // the process environment
	Bootstrap      map[string]string   // the bootstrap file's values
	DataDir        string
	Storage        proteusv1.Storage
	StorageFromEnv bool       // PROTEUS_DATABASE_URL is set
	EnvKeys        []auth.Key // from PROTEUS_API_KEYS
	SetupDone      bool       // loaded at startup
	SetupToken     string
	// OpenDatabase connects to the database described by bootstrap-style
	// values (PROTEUS_DATABASE_URL, _SCHEMA, _AUTO_MIGRATE), migrating when
	// migrate is set.
	OpenDatabase func(ctx context.Context, values map[string]string, migrate bool) (Database, error)
	// FileStore is where settings go without a database.
	FileStore func() settings.Store
	// Restart applies saved changes; it returns at once and the restart
	// happens after the current responses are written.
	Restart func()
}

// control is the API's view of Control plus the state that changes.
type control struct {
	*Control
	setupDone atomic.Bool
	tokens    *rate.Limiter
}

// WithControl enables the setup wizard and the admin pages.
func (a *API) WithControl(c *Control) *API {
	ctl := &control{Control: c, tokens: rate.NewLimiter(rate.Every(3*time.Second), 20)}
	ctl.setupDone.Store(c.SetupDone)
	a.ctl = ctl
	return a
}

// ReloadKeys reads the UI's keys from the store into the authenticator.
func (a *API) ReloadKeys(ctx context.Context) error {
	if a.ctl == nil {
		return nil
	}
	keys, err := a.ctl.Store.ListKeys(ctx)
	if err != nil {
		return err
	}
	a.auth.SetStored(authKeys(keys))
	return nil
}

func authKeys(keys []settings.APIKey) []auth.Key {
	out := make([]auth.Key, len(keys))
	for i, k := range keys {
		out[i] = auth.Key{ID: k.ID, Hash: k.Hash, Role: k.Role}
	}
	return out
}

// isSetup reports whether a procedure belongs to the setup wizard, which
// checks the setup token itself instead of an API key.
func isSetup(procedure string) bool {
	return strings.HasPrefix(procedure, "/"+proteusv1connect.SetupServiceName+"/")
}

// errNotSetUp is what every other call gets before the first key exists.
var errNotSetUp = connect.NewError(connect.CodeFailedPrecondition,
	"proteus-server is not set up yet: open /setup in a browser to finish setting it up")

var errNoKeys = connect.NewError(connect.CodeUnauthenticated,
	"proteus-server has no API keys: set PROTEUS_API_KEYS with an admin key")

// gate refuses API calls while no key exists, so a fresh server (or one
// whose keys were all removed from the environment) is never open, unless
// PROTEUS_ALLOW_NO_AUTH says so.
func (a *API) gate() error {
	switch {
	case a.ctl == nil || a.auth.Enabled() || a.cfg.AllowNoAuth:
		return nil
	case !a.ctl.setupDone.Load():
		return errNotSetUp
	}
	return errNoKeys
}

// --- SetupService ---

type setupService struct{ api *API }

var _ proteusv1connect.SetupServiceHandler = (*setupService)(nil)

func (s *setupService) ctl() (*control, error) {
	if s.api.ctl == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, "setup is not available on this server")
	}
	return s.api.ctl, nil
}

func (s *setupService) GetSetupStatus(ctx context.Context, _ *proteusv1.GetSetupStatusRequest) (*proteusv1.GetSetupStatusResponse, error) {
	c, err := s.ctl()
	if err != nil {
		return nil, err
	}
	d, err := c.Store.Load(ctx)
	if err != nil {
		return nil, s.api.internal(ctx, err)
	}
	return &proteusv1.GetSetupStatusResponse{
		NeedsSetup:             !d.SetupDone(),
		Storage:                c.Storage,
		StorageFromEnvironment: c.StorageFromEnv,
		HasEnvironmentKeys:     len(c.EnvKeys) > 0,
		Version:                Version,
	}, nil
}

// checkToken guards every wizard call that does anything: the setup must
// still be pending and the token must match.
func (s *setupService) checkToken(ctx context.Context, token string) (*control, error) {
	c, err := s.ctl()
	if err != nil {
		return nil, err
	}
	if !c.tokens.Allow() {
		return nil, connect.NewError(connect.CodeResourceExhausted, "too many attempts; wait a minute and try again")
	}
	d, err := c.Store.Load(ctx)
	if err != nil {
		return nil, s.api.internal(ctx, err)
	}
	if d.SetupDone() {
		return nil, connect.NewError(connect.CodeFailedPrecondition, "setup is already complete; sign in with an admin API key")
	}
	if subtle.ConstantTimeCompare([]byte(strings.TrimSpace(token)), []byte(c.SetupToken)) != 1 || c.SetupToken == "" {
		return nil, connect.NewError(connect.CodePermissionDenied, "wrong setup token: copy it from the server log (the line \"setup token\")")
	}
	return c, nil
}

func (s *setupService) CheckSetupToken(ctx context.Context, req *proteusv1.CheckSetupTokenRequest) (*proteusv1.CheckSetupTokenResponse, error) {
	if _, err := s.checkToken(ctx, req.GetSetupToken()); err != nil {
		return nil, err
	}
	return &proteusv1.CheckSetupTokenResponse{}, nil
}

func databaseValues(db *proteusv1.Database) (map[string]string, error) {
	if strings.TrimSpace(db.GetUrl()) == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, "database url is required")
	}
	m := map[string]string{"PROTEUS_DATABASE_URL": db.GetUrl(), "PROTEUS_DATABASE_SCHEMA": db.GetSchema()}
	if db.AutoMigrate != nil {
		m["PROTEUS_AUTO_MIGRATE"] = strconv.FormatBool(db.GetAutoMigrate())
	}
	clean, err := config.ValidateBootstrap(m)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err.Error())
	}
	return clean, nil
}

func (s *setupService) TestDatabase(ctx context.Context, req *proteusv1.TestDatabaseRequest) (*proteusv1.TestDatabaseResponse, error) {
	c, err := s.checkToken(ctx, req.GetSetupToken())
	if err != nil {
		return nil, err
	}
	values, err := databaseValues(req.GetDatabase())
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	db, err := c.OpenDatabase(ctx, values, false)
	if err != nil {
		return &proteusv1.TestDatabaseResponse{Error: err.Error()}, nil
	}
	defer db.Close()
	v, err := db.ServerVersion(ctx)
	if err != nil {
		return &proteusv1.TestDatabaseResponse{Error: err.Error()}, nil
	}
	return &proteusv1.TestDatabaseResponse{Ok: true, ServerVersion: "PostgreSQL " + v}, nil
}

func (s *setupService) CompleteSetup(ctx context.Context, req *proteusv1.CompleteSetupRequest) (*proteusv1.CompleteSetupResponse, error) {
	c, err := s.checkToken(ctx, req.GetSetupToken())
	if err != nil {
		return nil, err
	}
	name := req.GetAdminKeyName()
	if strings.TrimSpace(name) == "" {
		name = "Admin"
	}
	if name, err = settings.CleanName(name); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, "admin key name: "+err.Error())
	}

	// Where the settings go: the database from the environment, a database
	// chosen here (remembered in the bootstrap file), or the data folder.
	target := c.Store
	var bootstrap map[string]string
	switch {
	case c.StorageFromEnv:
	case req.GetStorage() == proteusv1.Storage_STORAGE_POSTGRES:
		if bootstrap, err = databaseValues(req.GetDatabase()); err != nil {
			return nil, err
		}
		octx, cancel := context.WithTimeout(ctx, 60*time.Second)
		db, err := c.OpenDatabase(octx, bootstrap, true)
		cancel()
		if err != nil {
			return nil, connect.NewError(connect.CodeFailedPrecondition, "database: "+err.Error())
		}
		defer db.Close()
		target = db.Settings()
	default:
		bootstrap = map[string]string{}
		target = c.FileStore()
	}

	clean, err := config.ValidateSaved(c.Env, bootstrap, req.GetSettings())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err.Error())
	}
	cur, err := target.Load(ctx)
	if err != nil {
		return nil, s.api.internal(ctx, err)
	}
	if cur.SetupDone() {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			"that database was already set up by another proteus server: point this one at it with PROTEUS_DATABASE_URL and use an admin key from it")
	}
	// The bootstrap file first: if anything below fails, the next attempt
	// finds the same unfinished store.
	if bootstrap != nil && !maps.Equal(bootstrap, c.Bootstrap) {
		if err := settings.WriteBootstrap(c.DataDir, bootstrap); err != nil {
			return nil, connect.NewError(connect.CodeFailedPrecondition, err.Error()+" (set PROTEUS_DATA_DIR to a writable folder)")
		}
	}
	key, secret := newKey(name, settings.RoleAdmin)
	if err := target.CreateKey(ctx, key); err != nil {
		return nil, s.api.storeErr(ctx, err)
	}
	if _, err := target.Save(ctx, settings.Doc{Values: clean, SetupCompletedAt: time.Now().UTC()}, cur.Revision); err != nil {
		_ = target.DeleteKey(context.WithoutCancel(ctx), key.ID)
		return nil, s.api.storeErr(ctx, err)
	}
	c.setupDone.Store(true)
	s.api.log.Info("setup complete", "admin_key", key.ID, "storage", req.GetStorage().String())
	c.Restart()
	return &proteusv1.CompleteSetupResponse{AdminKey: keyProto(key, false), Secret: secret}, nil
}

// newKey makes a key with a fresh id; the plain key is returned once.
func newKey(name, role string) (settings.APIKey, string) {
	secret := auth.NewKey()
	var b [8]byte
	_, _ = rand.Read(b[:])
	id := "k_" + strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b[:]))
	return settings.APIKey{
		ID: id, Name: name, Role: role, Hash: settings.Hash(secret), Prefix: secret[:min(len(secret), 10)],
		CreatedAt: time.Now().UTC().Truncate(time.Millisecond),
	}, secret
}

func roleProto(r string) proteusv1.ApiKey_Role {
	if r == settings.RoleAdmin {
		return proteusv1.ApiKey_ROLE_ADMIN
	}
	return proteusv1.ApiKey_ROLE_CLIENT
}

func keyProto(k settings.APIKey, env bool) *proteusv1.ApiKey {
	p := &proteusv1.ApiKey{Id: k.ID, Name: k.Name, Role: roleProto(k.Role), Prefix: k.Prefix, FromEnvironment: env}
	if !k.CreatedAt.IsZero() {
		p.CreateTime = timestamppb.New(k.CreatedAt)
	}
	return p
}

// internal hides the cause from the client and logs it.
func (a *API) internal(ctx context.Context, err error) error {
	a.log.Error("settings store", "request_id", requestIDFrom(ctx), "error", err)
	return connect.NewError(connect.CodeInternal, "internal error")
}

func (a *API) storeErr(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, settings.ErrRevisionMismatch):
		return connect.NewError(connect.CodeAborted, err.Error())
	case errors.Is(err, settings.ErrNotFound):
		return connect.NewError(connect.CodeNotFound, err.Error())
	case errors.Is(err, settings.ErrKeyExists):
		return connect.NewError(connect.CodeAlreadyExists, err.Error())
	}
	return a.internal(ctx, err)
}

// --- AdminService ---

type adminService struct{ api *API }

var _ proteusv1connect.AdminServiceHandler = (*adminService)(nil)

func (s *adminService) ctl(ctx context.Context) (*control, error) {
	if s.api.ctl == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, "settings are not available on this server")
	}
	if !auth.IsAdmin(ctx) {
		return nil, connect.NewError(connect.CodePermissionDenied, "this needs an API key with the admin role")
	}
	return s.api.ctl, nil
}

var kinds = map[config.Kind]proteusv1.Setting_Kind{
	config.String: proteusv1.Setting_KIND_STRING, config.Int: proteusv1.Setting_KIND_INT,
	config.Float: proteusv1.Setting_KIND_FLOAT, config.Bool: proteusv1.Setting_KIND_BOOL,
	config.Duration: proteusv1.Setting_KIND_DURATION, config.Bytes: proteusv1.Setting_KIND_BYTES,
	config.List: proteusv1.Setting_KIND_LIST, config.Secret: proteusv1.Setting_KIND_SECRET,
}

func (s *adminService) GetSettings(ctx context.Context, _ *proteusv1.GetSettingsRequest) (*proteusv1.GetSettingsResponse, error) {
	c, err := s.ctl(ctx)
	if err != nil {
		return nil, err
	}
	d, err := c.Store.Load(ctx)
	if err != nil {
		return nil, s.api.internal(ctx, err)
	}
	out := make([]*proteusv1.Setting, 0, len(config.Fields))
	for _, f := range config.Fields {
		st := &proteusv1.Setting{
			Name: f.Name, Group: f.Group, Label: f.Label, Help: f.Help, Kind: kinds[f.Kind],
			DefaultValue: f.Default, Editable: f.Editable, Source: proteusv1.Setting_SOURCE_DEFAULT, Value: f.Default,
		}
		if f.Kind == config.Secret {
			st.DefaultValue = ""
		}
		for _, l := range []struct {
			v   string
			src proteusv1.Setting_Source
		}{
			{strings.TrimSpace(c.Env(f.Name)), proteusv1.Setting_SOURCE_ENVIRONMENT},
			{c.Bootstrap[f.Name], proteusv1.Setting_SOURCE_BOOTSTRAP},
			{d.Values[f.Name], proteusv1.Setting_SOURCE_SAVED},
		} {
			if l.v != "" {
				st.Value, st.Source = l.v, l.src
				break
			}
		}
		if f.Kind == config.Secret {
			st.IsSet = st.Value != ""
			st.Value = ""
		}
		out = append(out, st)
	}
	return &proteusv1.GetSettingsResponse{Settings: out, Revision: d.Revision, Storage: c.Storage, DataDir: c.DataDir}, nil
}

func (s *adminService) UpdateSettings(ctx context.Context, req *proteusv1.UpdateSettingsRequest) (*proteusv1.UpdateSettingsResponse, error) {
	c, err := s.ctl(ctx)
	if err != nil {
		return nil, err
	}
	d, err := c.Store.Load(ctx)
	if err != nil {
		return nil, s.api.internal(ctx, err)
	}
	if req.GetRevision() != d.Revision {
		return nil, connect.NewError(connect.CodeAborted, settings.ErrRevisionMismatch.Error())
	}
	merged := maps.Clone(d.Values)
	if merged == nil {
		merged = map[string]string{}
	}
	maps.Copy(merged, req.GetValues())
	clean, err := config.ValidateSaved(c.Env, c.Bootstrap, merged)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err.Error())
	}
	if maps.Equal(clean, d.Values) || (len(clean) == 0 && len(d.Values) == 0) {
		return &proteusv1.UpdateSettingsResponse{Revision: d.Revision}, nil
	}
	d.Values = clean
	saved, err := c.Store.Save(ctx, d, d.Revision)
	if err != nil {
		return nil, s.api.storeErr(ctx, err)
	}
	s.api.log.Info("settings saved", "caller", auth.Caller(ctx), "revision", saved.Revision)
	c.Restart()
	return &proteusv1.UpdateSettingsResponse{Revision: saved.Revision, Restarting: true}, nil
}

func (s *adminService) ListApiKeys(ctx context.Context, _ *proteusv1.ListApiKeysRequest) (*proteusv1.ListApiKeysResponse, error) {
	c, err := s.ctl(ctx)
	if err != nil {
		return nil, err
	}
	keys, err := c.Store.ListKeys(ctx)
	if err != nil {
		return nil, s.api.internal(ctx, err)
	}
	out := make([]*proteusv1.ApiKey, 0, len(keys)+len(c.EnvKeys))
	for _, k := range c.EnvKeys {
		out = append(out, keyProto(settings.APIKey{ID: k.ID, Name: k.ID, Role: k.Role}, true))
	}
	for _, k := range keys {
		out = append(out, keyProto(k, false))
	}
	return &proteusv1.ListApiKeysResponse{Keys: out, CallerId: auth.Caller(ctx)}, nil
}

func (s *adminService) CreateApiKey(ctx context.Context, req *proteusv1.CreateApiKeyRequest) (*proteusv1.CreateApiKeyResponse, error) {
	c, err := s.ctl(ctx)
	if err != nil {
		return nil, err
	}
	name, err := settings.CleanName(req.GetName())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err.Error())
	}
	role := settings.RoleClient
	if req.GetRole() == proteusv1.ApiKey_ROLE_ADMIN {
		role = settings.RoleAdmin
	}
	key, secret := newKey(name, role)
	if err := c.Store.CreateKey(ctx, key); err != nil {
		return nil, s.api.storeErr(ctx, err)
	}
	if err := s.api.ReloadKeys(ctx); err != nil {
		return nil, s.api.internal(ctx, err)
	}
	s.api.log.Info("api key created", "caller", auth.Caller(ctx), "key", key.ID, "role", role)
	return &proteusv1.CreateApiKeyResponse{Key: keyProto(key, false), Secret: secret}, nil
}

func (s *adminService) DeleteApiKey(ctx context.Context, req *proteusv1.DeleteApiKeyRequest) (*proteusv1.DeleteApiKeyResponse, error) {
	c, err := s.ctl(ctx)
	if err != nil {
		return nil, err
	}
	id := req.GetId()
	for _, k := range c.EnvKeys {
		if k.ID == id {
			return nil, connect.NewError(connect.CodeFailedPrecondition, "this key comes from PROTEUS_API_KEYS; remove it there")
		}
	}
	if id == auth.Caller(ctx) {
		return nil, connect.NewError(connect.CodeFailedPrecondition, "you cannot revoke the key you are signed in with")
	}
	keys, err := c.Store.ListKeys(ctx)
	if err != nil {
		return nil, s.api.internal(ctx, err)
	}
	found := false
	for _, k := range keys {
		if k.ID == id {
			found = true
			if k.Role == settings.RoleAdmin && countAdmins(keys) == 1 {
				return nil, connect.NewError(connect.CodeFailedPrecondition, "this is the last admin key; create another admin key first")
			}
		}
	}
	if !found {
		return nil, connect.NewError(connect.CodeNotFound, settings.ErrNotFound.Error())
	}
	if err := c.Store.DeleteKey(ctx, id); err != nil {
		return nil, s.api.storeErr(ctx, err)
	}
	if err := s.api.ReloadKeys(ctx); err != nil {
		return nil, s.api.internal(ctx, err)
	}
	s.api.log.Info("api key revoked", "caller", auth.Caller(ctx), "key", id)
	return &proteusv1.DeleteApiKeyResponse{}, nil
}

func countAdmins(keys []settings.APIKey) int {
	n := 0
	for _, k := range keys {
		if k.Role == settings.RoleAdmin {
			n++
		}
	}
	return n
}
