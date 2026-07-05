package ociregistry

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func extractLayer(image, digest, dest, token string) error {
	// Skip if already extracted
	if _, err := os.Stat(filepath.Join(dest, ".done")); err == nil {
		return nil
	}

	if err := os.MkdirAll(dest, 0755); err != nil {
		return err
	}

	url := fmt.Sprintf("https://registry-1.docker.io/v2/%s/blobs/%s", image, digest)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("registry returned %d", resp.StatusCode)
	}

	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return err
	}
	defer gz.Close()

	if err := untar(tar.NewReader(gz), dest); err != nil {
		return err
	}

	done, err := os.Create(filepath.Join(dest, ".done"))
	if err != nil {
		return err
	}
	done.Close()
	return nil
}

func untar(tr *tar.Reader, dest string) error {
	// Directory modes are applied in a second pass: a restrictive mode (e.g.
	// 0555) must not stop us writing the directory's children first.
	type dirMode struct {
		path string
		mode os.FileMode
	}
	var dirs []dirMode

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Whiteout: .wh. prefix means delete the file from a lower layer
		base := filepath.Base(hdr.Name)
		if strings.HasPrefix(base, ".wh.") {
			target := filepath.Join(dest, filepath.Dir(hdr.Name), strings.TrimPrefix(base, ".wh."))
			os.RemoveAll(target)
			continue
		}

		target := filepath.Join(dest, filepath.Clean(hdr.Name))
		// Guard against path traversal
		if !strings.HasPrefix(target, filepath.Clean(dest)+string(os.PathSeparator)) {
			continue
		}

		// hdr.FileInfo().Mode() converts the raw tar mode into an os.FileMode
		// with the setuid/setgid/sticky bits in the right places; hdr.Mode
		// alone would misplace them. We chmod explicitly because MkdirAll /
		// OpenFile apply the umask and ignore the special bits.
		mode := hdr.FileInfo().Mode()

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			dirs = append(dirs, dirMode{target, mode})
		case tar.TypeReg:
			if err := writeFile(tr, target, mode); err != nil {
				return err
			}
		case tar.TypeSymlink:
			os.MkdirAll(filepath.Dir(target), 0755)
			os.Remove(target)
			os.Symlink(hdr.Linkname, target)
		case tar.TypeLink:
			os.MkdirAll(filepath.Dir(target), 0755)
			os.Link(filepath.Join(dest, filepath.Clean(hdr.Linkname)), target)
		}
	}

	// Apply directory modes now that all children exist.
	for _, d := range dirs {
		if err := os.Chmod(d.path, d.mode); err != nil {
			return err
		}
	}
	return nil
}

func writeFile(r io.Reader, path string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode.Perm())
	if err != nil {
		return err
	}
	_, err = io.Copy(f, r)
	f.Close()
	if err != nil {
		return err
	}
	// Set the exact mode (setuid/setgid/sticky + perms) free of the umask.
	return os.Chmod(path, mode)
}
