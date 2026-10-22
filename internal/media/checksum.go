package media

import (
	"crypto/md5" //nolint:gosec // App Store Connect identifies uploads by MD5.
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"io"
	"os"

	"github.com/nikeee/store-cli/internal/clierr"
)

// ChecksumList holds lowercase hex digests in the order of the input files.
type ChecksumList []string

// MD5List returns the MD5 of each file, in order.
func MD5List(paths []string) (ChecksumList, error) { return sums(paths, md5.New) } //nolint:gosec

// SHA256List returns the SHA-256 of each file, in order.
func SHA256List(paths []string) (ChecksumList, error) { return sums(paths, sha256.New) }

func sums(paths []string, newHash func() hash.Hash) (ChecksumList, error) {
	out := make(ChecksumList, 0, len(paths))
	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			return nil, clierr.New("%v", err)
		}
		h := newHash()
		_, err = io.Copy(h, f)
		_ = f.Close()
		if err != nil {
			return nil, clierr.New("%v", err)
		}
		out = append(out, hex.EncodeToString(h.Sum(nil)))
	}
	return out, nil
}
