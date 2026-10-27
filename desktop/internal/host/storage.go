package host

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
)

// Storage exclusively owns the community app's data directory. It never selects
// the purchased application's directory or stores the database recovery key.
type Storage struct {
	Directory  string
	SigningKey string
	lock       *os.File
	once       sync.Once
	closeErr   error
}

func OpenStorage(directory string) (*Storage, error) {
	if directory == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return nil, err
		}
		directory = filepath.Join(base, "FastenCommunity")
	}
	dir, err := filepath.Abs(directory)
	if err != nil {
		return nil, err
	}
	if stat, err := os.Lstat(dir); err == nil && stat.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("data directory must not be a symlink")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	stat, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if stat.Mode().Perm()&0077 != 0 {
		return nil, errors.New("data directory must be private (mode 0700)")
	}
	lockPath := filepath.Join(dir, ".desktop.lock")
	if stat, err := os.Lstat(lockPath); err == nil && stat.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("lock file must not be a symlink")
	}
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		lock.Close()
		return nil, fmt.Errorf("data directory is already open: %w", err)
	}
	storage := &Storage{Directory: dir, lock: lock}
	keyPath := filepath.Join(dir, "jwt.key")
	if stat, err := os.Lstat(keyPath); err == nil && (!stat.Mode().IsRegular() || stat.Mode().Perm()&0077 != 0) {
		storage.Close()
		return nil, errors.New("signing key must be a private regular file")
	}
	key, err := os.ReadFile(keyPath)
	if os.IsNotExist(err) {
		raw := make([]byte, 48)
		if _, err = rand.Read(raw); err != nil {
			storage.Close()
			return nil, err
		}
		key = []byte(base64.RawURLEncoding.EncodeToString(raw))
		var f *os.File
		f, err = os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err == nil {
			_, err = f.Write(key)
			if err == nil {
				err = f.Sync()
			}
			closeErr := f.Close()
			if err == nil {
				err = closeErr
			}
		}
	}
	if err != nil {
		storage.Close()
		return nil, err
	}
	if len(strings.TrimSpace(string(key))) < 32 {
		storage.Close()
		return nil, errors.New("stored signing key is invalid; refusing to replace it")
	}
	storage.SigningKey = string(key)
	return storage, nil
}

func (s *Storage) Close() error {
	s.once.Do(func() {
		s.closeErr = syscall.Flock(int(s.lock.Fd()), syscall.LOCK_UN)
		err := s.lock.Close()
		if s.closeErr == nil {
			s.closeErr = err
		}
	})
	return s.closeErr
}
