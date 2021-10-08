package azuresecrets

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/vault-plugin-secrets-azure/api"
	"github.com/hashicorp/vault-plugin-secrets-azure/ticker"
	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/helper/locksutil"
	"github.com/hashicorp/vault/sdk/logical"
)

const (
	rootCredTickerID = "root-creds"
)

type azureSecretBackend struct {
	*framework.Backend

	getProvider func(*clientSettings, bool, api.Passwords) (api.AzureProvider, error)
	client      *client
	settings    *clientSettings
	lock        sync.RWMutex

	// Creating/deleting passwords against a single Application is a PATCH
	// operation that must be locked per Application Object ID.
	appLocks []*locksutil.LockEntry

	ticker *ticker.Ticker
}

func Factory(ctx context.Context, conf *logical.BackendConfig) (logical.Backend, error) {
	b := backend()
	if err := b.Setup(ctx, conf); err != nil {
		return nil, err
	}

	// Need to set up the ticker after calling Setup() so we can reference the logger
	ticker, err := ticker.NewTicker(b.Logger(), 0)
	if err != nil {
		return nil, fmt.Errorf("failed to set up asynchronous ticker: %w", err)
	}
	b.ticker = ticker

	return b, nil
}

func backend() *azureSecretBackend {
	var b = azureSecretBackend{}

	b.Backend = &framework.Backend{
		Help: strings.TrimSpace(backendHelp),
		PathsSpecial: &logical.Paths{
			SealWrapStorage: []string{
				"config",
			},
		},
		Paths: framework.PathAppend(
			pathsRole(&b),
			[]*framework.Path{
				pathConfig(&b),
				pathServicePrincipal(&b),
				pathRotateRoot(&b),
			},
		),
		Secrets: []*framework.Secret{
			secretServicePrincipal(&b),
			secretStaticServicePrincipal(&b),
		},
		BackendType: logical.TypeLogical,
		Invalidate:  b.invalidate,

		WALRollback: b.walRollback,

		// Role assignment can take up to a few minutes, so ensure we don't try
		// to roll back during creation.
		WALRollbackMinAge: 10 * time.Minute,
		InitializeFunc:    b.initialize,
		Clean:             b.cleanup,
	}

	b.getProvider = newAzureProvider
	b.appLocks = locksutil.CreateLocks()

	return &b
}

// reset clears the backend's cached client
// This is used when the configuration changes and a new client should be
// created with the updated settings.
func (b *azureSecretBackend) reset() {
	b.lock.Lock()
	defer b.lock.Unlock()

	b.settings = nil
	b.client = nil
}

func (b *azureSecretBackend) invalidate(ctx context.Context, key string) {
	switch key {
	case "config":
		b.reset()
	}
}

func (b *azureSecretBackend) getClient(ctx context.Context, s logical.Storage) (*client, error) {
	b.lock.RLock()

	if b.client.Valid() {
		b.lock.RUnlock()
		return b.client, nil
	}

	b.lock.RUnlock()
	b.lock.Lock()
	defer b.lock.Unlock()

	if b.client.Valid() {
		return b.client, nil
	}

	config, err := b.getConfig(ctx, s)
	if err != nil {
		return nil, err
	}

	if b.settings == nil {
		if config == nil {
			config = new(azureConfig)
		}

		settings, err := b.getClientSettings(ctx, config)
		if err != nil {
			return nil, err
		}
		b.settings = settings
	}

	passwords := api.Passwords{
		PolicyGenerator: b.System(),
		PolicyName:      config.PasswordPolicy,
	}

	p, err := b.getProvider(b.settings, config.UseMsGraphAPI, passwords)
	if err != nil {
		return nil, err
	}

	c := &client{
		provider:   p,
		settings:   b.settings,
		expiration: time.Now().Add(clientLifetime),
		passwords:  passwords,
	}
	b.client = c

	return c, nil
}

func (b *azureSecretBackend) initialize(ctx context.Context, req *logical.InitializationRequest) error {
	// Set up automatic rotation logic
	cfg, err := b.getConfig(ctx, req.Storage)
	if err != nil {
		return fmt.Errorf("failed to retrieve configuration: %w", err)
	}
	// No config exists, don't set up the ticker
	if cfg == nil {
		return nil
	}

	// No regular rotation set, also don't set up the ticker
	if cfg.NextRootRotationTime.IsZero() {
		return nil
	}

	// TODO: Set up auto root rotation when saving the config
	// Make sure it handles updating auto root rotation too
	// On update: Next run should be when? Immediate?
	firstRun := cfg.NextRootRotationTime
	_, err = b.ticker.Run(cfg.RootRotationCadence, b.automaticRotateRootFunc(req.Storage),
		ticker.ID(rootCredTickerID),
		ticker.FirstRun(firstRun),
	)
	return err
}

func (b *azureSecretBackend) cleanup(ctx context.Context) {
	err := b.ticker.Close()
	if err != nil {
		b.Logger().Error("Not all goroutines closed cleanly", "error", err)
	}
}

const backendHelp = `
The Azure secrets backend dynamically generates Azure service
principals. The SP credentials have a configurable lease and
are automatically revoked at the end of the lease.

After mounting this backend, credentials to manage Azure resources
must be configured with the "config/" endpoints and policies must be
written using the "roles/" endpoints before any credentials can be
generated.
`
