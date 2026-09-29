package history

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/reinaldosaraiva/gocouple/internal/config"
	"github.com/reinaldosaraiva/gocouple/internal/model"
)

// ConfigHash returns the first 12 hex digits of a sha256 over everything that
// changes an analysis result: tool version, effective config and load flags.
func ConfigHash(toolVersion string, cfg config.Config, includeTests bool, patterns []string) string {
	blob, err := json.Marshal(struct {
		Tool         string
		Config       config.Config
		IncludeTests bool
		Patterns     []string
	}{toolVersion, cfg, includeTests, patterns})
	if err != nil {
		blob = []byte(toolVersion)
	}
	sum := sha256.Sum256(blob)
	return hex.EncodeToString(sum[:])[:12]
}

type cache struct {
	dir  string
	hash string
}

func (c cache) path(sha string) string {
	return filepath.Join(c.dir, sha+"-"+c.hash+".json")
}

// load returns a cached snapshot; a missing or corrupt entry is a miss.
func (c cache) load(sha string) (*model.Snapshot, bool) {
	data, err := os.ReadFile(c.path(sha))
	if err != nil {
		return nil, false
	}
	var snap model.Snapshot
	if err := json.Unmarshal(data, &snap); err != nil || snap.SchemaVersion != model.SchemaVersion {
		return nil, false
	}
	return &snap, true
}

func (c cache) store(sha string, snap *model.Snapshot) error {
	if err := os.MkdirAll(c.dir, 0o755); err != nil {
		return fmt.Errorf("creating cache dir: %w", err)
	}
	data, err := json.Marshal(snap)
	if err != nil {
		return fmt.Errorf("encoding cache entry: %w", err)
	}
	tmp, err := os.CreateTemp(c.dir, strings.TrimSuffix(filepath.Base(c.path(sha)), ".json")+".*.tmp")
	if err != nil {
		return fmt.Errorf("creating cache entry: %w", err)
	}
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if err := errors.Join(werr, cerr); err != nil {
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("writing cache entry: %w", err)
	}
	if err := os.Rename(tmp.Name(), c.path(sha)); err != nil {
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("publishing cache entry: %w", err)
	}
	return nil
}
