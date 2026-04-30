package sandbox

import (
	"archive/tar"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/klauspost/compress/zstd"
)

// ListPaths returns all file paths in the zstd-compressed tarball.
func ListPaths(srcFile string) ([]string, error) {
	f, err := os.Open(srcFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	zr, err := zstd.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer zr.Close()

	tr := tar.NewReader(zr)
	var paths []string
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		paths = append(paths, header.Name)
	}
	return paths, nil
}

// Compress creates a zstd-compressed tarball of srcDir.
// The srcDir itself will be the top-level directory in the archive.
func Compress(srcDir, destFile string, level zstd.EncoderLevel) error {
	f, err := os.Create(destFile)
	if err != nil {
		return err
	}
	defer f.Close()

	zw, err := zstd.NewWriter(f, zstd.WithEncoderLevel(level))
	if err != nil {
		return err
	}
	defer zw.Close()

	tw := tar.NewWriter(zw)
	defer tw.Close()

	parentDir := filepath.Dir(srcDir)

	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(parentDir, path)
		if err != nil {
			return err
		}
		header.Name = relPath

		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			header.Linkname = link
		}

		if stat, ok := info.Sys().(*syscall.Stat_t); ok {
			header.Uid = int(stat.Uid)
			header.Gid = int(stat.Gid)
			// For block/char devices
			if info.Mode()&(os.ModeDevice|os.ModeCharDevice) != 0 {
				header.Devmajor = int64(stat.Rdev >> 8 & 0xfff)
				header.Devminor = int64(stat.Rdev & 0xff)
			}
		}

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if info.Mode().IsRegular() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()
			_, err = io.Copy(tw, file)
			return err
		}

		return nil
	})
}

// Extract extracts a zstd-compressed tarball to destDir.
func Extract(srcFile, destDir string) error {
	f, err := os.Open(srcFile)
	if err != nil {
		return err
	}
	defer f.Close()

	zr, err := zstd.NewReader(f)
	if err != nil {
		return err
	}
	defer zr.Close()

	tr := tar.NewReader(zr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(destDir, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			if err := os.Symlink(header.Linkname, target); err != nil {
				return err
			}
		case tar.TypeLink:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			if err := os.Link(filepath.Join(destDir, header.Linkname), target); err != nil {
				return err
			}
		case tar.TypeChar, tar.TypeBlock:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			mode := uint32(header.Mode)
			if header.Typeflag == tar.TypeChar {
				mode |= syscall.S_IFCHR
			} else {
				mode |= syscall.S_IFBLK
			}
			dev := int(header.Devmajor<<8 | header.Devminor)
			if err := syscall.Mknod(target, mode, dev); err != nil {
				return err
			}
		case tar.TypeFifo:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			if err := syscall.Mkfifo(target, uint32(header.Mode)); err != nil {
				return err
			}
		}

		// Restore ownership
		if err := os.Lchown(target, header.Uid, header.Gid); err != nil {
			return err
		}

		// Restore timestamps (not for symlinks as os.Chtimes follows them)
		if header.Typeflag != tar.TypeSymlink {
			_ = os.Chtimes(target, header.AccessTime, header.ModTime)
		}
	}
	return nil
}
