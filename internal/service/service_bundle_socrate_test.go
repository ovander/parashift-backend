package service_test

import (
	"io"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/ovander/parashift/internal/config"
	"github.com/ovander/parashift/internal/repo"
	"github.com/ovander/parashift/internal/service"
)

func newBundle(t *testing.T, cfg *config.Config) *service.ServiceBundle {
	t.Helper()
	sqlDB, _, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	gdb, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	require.NoError(t, err)
	lg := logrus.New()
	lg.SetOutput(io.Discard)
	b := service.NewServiceBundle(repo.NewRepoBundle(gdb), logrus.NewEntry(lg), cfg)
	t.Cleanup(b.Emitter.Close)
	return b
}

// The BFF needs both the Socrate client and the sign-in service: bootstrap
// turns the BFF off when either is nil, so a missing wire would silently
// disable sign-in.
func TestServiceBundle_WiresSocrateAndSessionAuth(t *testing.T) {
	cfg := &config.Config{Socrate: config.SocrateConfig{
		BaseURL: "https://socrate.example", AdminURL: "http://127.0.0.1:18082",
		ClientID: "client", ClientSecret: "secret", AppID: "7",
	}}
	b := newBundle(t, cfg)
	assert.NotNil(t, b.SocrateClient)
	assert.NotNil(t, b.SessionAuth)
}

func TestServiceBundle_NoSocrateNoSessionAuth(t *testing.T) {
	b := newBundle(t, &config.Config{})
	assert.Nil(t, b.SocrateClient)
	assert.Nil(t, b.SessionAuth)
}
