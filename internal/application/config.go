package application

import (
	"crypto/rand"
	"encoding/hex"
	"soundledger/internal/audit"
	"soundledger/internal/store"
	"time"
)

type Clock func() time.Time
type IDFactory func() string

type Config struct {
	Store          *store.FileStore
	Clock          Clock
	IDFactory      IDFactory
	MaxUploadBytes int64
}

type Service struct {
	store          *store.FileStore
	clock          Clock
	ids            IDFactory
	evidence       *audit.EvidenceService
	mailboxes      *mailboxRegistry
	maxUploadBytes int64
}

func New(config Config) (*Service, error) {
	if config.Store == nil {
		return nil, fmtError("存储不能为空")
	}
	if config.Clock == nil {
		config.Clock = time.Now
	}
	if config.IDFactory == nil {
		config.IDFactory = randomID
	}
	if config.MaxUploadBytes <= 0 {
		config.MaxUploadBytes = 64 << 20
	}
	return &Service{store: config.Store, clock: config.Clock, ids: config.IDFactory, evidence: audit.NewEvidenceService(config.Clock, config.IDFactory), mailboxes: newMailboxRegistry(), maxUploadBytes: config.MaxUploadBytes}, nil
}

func randomID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return hex.EncodeToString([]byte(time.Now().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(b)
}

type applicationError string

func (e applicationError) Error() string { return string(e) }
func fmtError(message string) error      { return applicationError(message) }
