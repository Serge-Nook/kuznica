package deb

import (
	"archive/tar"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

// ErrUnsupportedCompression is returned for archive members whose
// compression format КУЗНИЦА cannot read.
type ErrUnsupportedCompression struct{ Name string }

func (e ErrUnsupportedCompression) Error() string {
	return fmt.Sprintf("unsupported compression format: %s", e.Name)
}

// decompress wraps r with the decompressor matching the member name
// (data.tar.gz, data.tar.xz, data.tar.zst, data.tar.bz2 or data.tar).
// The returned closer must always be called.
func decompress(name string, r io.Reader) (io.Reader, io.Closer, error) {
	switch {
	case strings.HasSuffix(name, ".tar"):
		return r, nopCloser{}, nil
	case strings.HasSuffix(name, ".gz"):
		zr, err := gzip.NewReader(r)
		if err != nil {
			return nil, nopCloser{}, err
		}
		return zr, zr, nil
	case strings.HasSuffix(name, ".xz"):
		xr, err := xz.NewReader(r)
		if err != nil {
			return nil, nopCloser{}, err
		}
		return xr, nopCloser{}, nil
	case strings.HasSuffix(name, ".zst"):
		zr, err := zstd.NewReader(r)
		if err != nil {
			return nil, nopCloser{}, err
		}
		rc := zr.IOReadCloser()
		return rc, rc, nil
	case strings.HasSuffix(name, ".bz2"):
		return bzip2.NewReader(r), nopCloser{}, nil
	default:
		return nil, nopCloser{}, ErrUnsupportedCompression{Name: name}
	}
}

type nopCloser struct{}

func (nopCloser) Close() error { return nil }

// FileEntry describes a single member of the data archive.
type FileEntry struct {
	Path     string
	Size     int64
	Mode     os.FileMode
	IsDir    bool
	IsLink   bool
	LinkDest string
}

// extractTar unpacks a tar stream into dest and returns the list of members.
// Every member path is validated so that a malicious archive cannot escape
// the destination directory (path traversal / zip slip).
func extractTar(r io.Reader, dest string) ([]FileEntry, error) {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return nil, err
	}
	root, err := filepath.Abs(dest)
	if err != nil {
		return nil, err
	}

	var entries []FileEntry
	tr := tar.NewReader(r)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return entries, fmt.Errorf("read tar: %w", err)
		}

		clean, err := safeJoin(root, header.Name)
		if err != nil {
			return entries, err
		}

		entry := FileEntry{
			Path:  normalizeMemberName(header.Name),
			Size:  header.Size,
			Mode:  header.FileInfo().Mode(),
			IsDir: header.Typeflag == tar.TypeDir,
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(clean, 0o755); err != nil {
				return entries, err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(clean), 0o755); err != nil {
				return entries, err
			}
			if err := writeFile(clean, tr, header.FileInfo().Mode()); err != nil {
				return entries, err
			}
		case tar.TypeSymlink, tar.TypeLink:
			entry.IsLink = true
			entry.LinkDest = header.Linkname
			if err := writeLink(root, clean, header); err != nil {
				return entries, err
			}
		default:
			// Character devices, fifos and similar members are recorded but
			// never materialised: КУЗНИЦА never runs as root.
		}

		if entry.Path != "" && entry.Path != "." {
			entries = append(entries, entry)
		}
	}
	return entries, nil
}

func writeFile(path string, r io.Reader, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode.Perm()|0o200)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return err
	}
	return nil
}

func writeLink(root, path string, header *tar.Header) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	target := header.Linkname
	if filepath.IsAbs(target) {
		// Absolute links point into the future installation root and are
		// recreated verbatim inside the staging directory.
		if _, err := safeJoin(root, target); err != nil {
			return err
		}
	} else {
		if _, err := safeJoin(filepath.Dir(path), target); err != nil {
			return err
		}
	}
	os.Remove(path)
	return os.Symlink(target, path)
}

// safeJoin joins name onto root and rejects any result that escapes root.
func safeJoin(root, name string) (string, error) {
	cleaned := filepath.Clean(filepath.Join(root, normalizeMemberName(name)))
	rootWithSep := strings.TrimSuffix(root, string(os.PathSeparator)) + string(os.PathSeparator)
	if cleaned != strings.TrimSuffix(root, string(os.PathSeparator)) && !strings.HasPrefix(cleaned, rootWithSep) {
		return "", fmt.Errorf("%w: %s", ErrPathTraversal, name)
	}
	return cleaned, nil
}

// normalizeMemberName turns "./usr/bin/foo" into "usr/bin/foo".
func normalizeMemberName(name string) string {
	name = strings.TrimPrefix(filepath.ToSlash(name), "./")
	name = strings.TrimPrefix(name, "/")
	return strings.TrimSuffix(name, "/")
}
