package sandbox

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/klauspost/compress/zstd"
	"github.com/pkg/xattr"
)

// ListPaths returns all file paths in the zstd-compressed tarball.
func ListPaths(srcFile string) ([]string, error) {
	f, err := os.OpenFile(srcFile, os.O_RDONLY, 0)
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
func Compress(srcDir, destFile string, level zstd.EncoderLevel) error {
	f, err := os.OpenFile(destFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o666)
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

	seenFiles := map[struct{ dev, ino uint64 }]string{}

	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if path == srcDir {
			return nil
		}

		var realPath string
		if info.Mode()&os.ModeSymlink != 0 {
			link, e := os.Readlink(path)
			if e != nil {
				return e
			}
			realPath = link
		}

		header, err := tar.FileInfoHeader(info, realPath)
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		header.Name = relPath

		if stat, ok := info.Sys().(*syscall.Stat_t); ok {
			header.Uid = int(stat.Uid)
			header.Gid = int(stat.Gid)

			// Handle hard links
			if !info.IsDir() {
				key := struct{ dev, ino uint64 }{dev: uint64(stat.Dev), ino: stat.Ino}
				if firstPath, seen := seenFiles[key]; seen {
					header.Typeflag = tar.TypeLink
					header.Linkname = firstPath
					header.Size = 0
				} else {
					seenFiles[key] = relPath
				}
			}

			// For block/char devices
			if info.Mode()&(os.ModeDevice|os.ModeCharDevice) != 0 {
				header.Devmajor = int64(stat.Rdev >> 8 & 0xfff)
				header.Devminor = int64(stat.Rdev & 0xff)
			}
		}

		// Handle xattrs
		xattrs, err := xattr.LList(path)
		if err == nil && len(xattrs) > 0 {
			if header.PAXRecords == nil {
				header.PAXRecords = make(map[string]string)
			}
			for _, attr := range xattrs {
				val, err := xattr.LGet(path, attr)
				if err == nil {
					header.PAXRecords["SCHILY.xattr."+attr] = string(val)
				}
			}
		}

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if header.Typeflag == tar.TypeReg {
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
	f, err := os.OpenFile(srcFile, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()

	if e := os.MkdirAll(destDir, 0o755); e != nil {
		return e
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	if e := os.Chdir(destDir); e != nil {
		return e
	}
	defer os.Chdir(cwd)

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

		target := header.Name

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := os.Symlink(header.Linkname, target); err != nil {
				return err
			}
		case tar.TypeLink:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := os.Link(header.Linkname, target); err != nil {
				return err
			}
		case tar.TypeChar, tar.TypeBlock:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
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
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
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
			if err := os.Chtimes(target, header.AccessTime, header.ModTime); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to restore timestamps for %v: %v\n", target, err)
			}
		}

		// Restore xattrs
		for key, val := range header.PAXRecords {
			if attrName, ok := strings.CutPrefix(key, "SCHILY.xattr."); ok {
				if err := xattr.LSet(target, attrName, []byte(val)); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to restore xattr %v for %v: %v\n", attrName, target, err)
				}
			}
		}
	}
	return nil
}
