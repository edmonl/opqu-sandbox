package sandbox

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"github.com/edmonl/opqu-sandbox/internal/util"
	"github.com/klauspost/compress/zstd"
	"github.com/pkg/xattr"
	"golang.org/x/sys/unix"
)

// Compress creates a zstd-compressed tarball of srcDir.
func Compress(srcDir, destFile string, level zstd.EncoderLevel) (err error) {
	f, err := os.OpenFile(destFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o666)
	if err != nil {
		return fmt.Errorf("failed to create destination file %v: %w", destFile, err)
	}
	defer func() {
		if closeErr := f.Close(); err == nil && closeErr != nil {
			err = fmt.Errorf("failed to close destination file %v: %w", destFile, closeErr)
		}
	}()

	zw, err := zstd.NewWriter(f, zstd.WithEncoderLevel(level))
	if err != nil {
		return fmt.Errorf("failed to create zstd writer for %v: %w", destFile, err)
	}
	defer func() {
		if closeErr := zw.Close(); err == nil && closeErr != nil {
			err = fmt.Errorf("failed to finalize zstd archive %v: %w", destFile, closeErr)
		}
	}()

	tw := tar.NewWriter(zw)
	defer func() {
		if closeErr := tw.Close(); err == nil && closeErr != nil {
			err = fmt.Errorf("failed to finalize tar archive %v: %w", destFile, closeErr)
		}
	}()

	seenFiles := map[struct{ dev, ino uint64 }]string{}
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("failed to walk source path %v: %w", path, err)
		}

		if path == srcDir {
			return nil
		}

		var realPath string
		if info.Mode()&os.ModeSymlink != 0 {
			link, e := os.Readlink(path)
			if e != nil {
				return fmt.Errorf("failed to read symlink %v: %w", path, e)
			}
			realPath = link
		}

		header, err := tar.FileInfoHeader(info, realPath)
		if err != nil {
			return fmt.Errorf("failed to create tar header for %v: %w", path, err)
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return fmt.Errorf("failed to resolve archive path for %v relative to %v: %w", path, srcDir, err)
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
				header.Devmajor = int64(unix.Major(stat.Rdev))
				header.Devminor = int64(unix.Minor(stat.Rdev))
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
				if err != nil {
					util.Warn("failed to read xattr %v for %v: %v", attr, path, err)
					continue
				}
				header.PAXRecords["SCHILY.xattr."+attr] = string(val)
			}
		} else if err != nil {
			util.Warn("failed to list xattrs for %v: %v", path, err)
		}

		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("failed to write tar header for %v: %w", path, err)
		}

		if header.Typeflag == tar.TypeReg {
			file, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("failed to open source file %v: %w", path, err)
			}
			defer file.Close()

			if _, err := io.Copy(tw, file); err != nil {
				return fmt.Errorf("failed to write source file %v to archive %v: %w", path, destFile, err)
			}
		}

		return nil
	})
}

// Extract extracts a zstd-compressed tarball to destDir.
// destDir must not already exist; Extract creates it.
// Archive entries must be ordered with directories before their children.
// Parent directories are not created implicitly; each entry's immediate parent
// must already exist as a real directory, not a symlink. Symlink entries are
// preserved, but later entries cannot be extracted through them.
func Extract(srcFile, destDir string) error {
	f, err := os.Open(srcFile)
	if err != nil {
		return fmt.Errorf("failed to open archive %v: %w", srcFile, err)
	}
	defer f.Close()

	destDir, err = filepath.Abs(destDir)
	if err != nil {
		return fmt.Errorf("failed to resolve destination directory %v: %w", destDir, err)
	}
	if e := os.Mkdir(destDir, 0o755); e != nil {
		if errors.Is(e, fs.ErrExist) {
			return fmt.Errorf("destination directory %v already exists", destDir)
		}
		return fmt.Errorf("failed to make destination directory %v: %w", destDir, e)
	}

	zr, err := zstd.NewReader(f)
	if err != nil {
		return fmt.Errorf("failed to read zstd archive %v: %w", srcFile, err)
	}
	defer zr.Close()

	tr := tar.NewReader(zr)
	var dirs []dirMetadata

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar entry from %v: %w", srcFile, err)
		}

		target, err := archiveTargetPath(destDir, header.Name)
		if err != nil {
			return fmt.Errorf("failed to resolve archive entry %v: %w", header.Name, err)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := requireParentDir(target); err != nil {
				return fmt.Errorf("failed to extract directory %v: %w", header.Name, err)
			}
			if err := os.Mkdir(target, os.FileMode(header.Mode)|0o700); err != nil {
				return fmt.Errorf("failed to create directory %v: %w", target, err)
			}
			dirs = append(dirs, dirMetadata{target: target, header: *header})
			continue
		case tar.TypeReg:
			if err := requireParentDir(target); err != nil {
				return fmt.Errorf("failed to extract regular file %v: %w", header.Name, err)
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("failed to create regular file %v: %w", target, err)
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return fmt.Errorf("failed to write regular file %v: %w", target, err)
			}
			if err := out.Close(); err != nil {
				return fmt.Errorf("failed to close regular file %v: %w", target, err)
			}
		case tar.TypeSymlink:
			if err := requireParentDir(target); err != nil {
				return fmt.Errorf("failed to extract symlink %v: %w", header.Name, err)
			}
			if err := os.Symlink(header.Linkname, target); err != nil {
				return fmt.Errorf("failed to create symlink %v pointing to %v: %w", target, header.Linkname, err)
			}
		case tar.TypeLink:
			if err := requireParentDir(target); err != nil {
				return fmt.Errorf("failed to extract hard link %v: %w", header.Name, err)
			}
			linkTarget, err := archiveTargetPath(destDir, header.Linkname)
			if err != nil {
				return fmt.Errorf("failed to resolve hard link target %v for %v: %w", header.Linkname, header.Name, err)
			}
			if err := os.Link(linkTarget, target); err != nil {
				return fmt.Errorf("failed to create hard link %v linked to %v: %w", target, linkTarget, err)
			}
		case tar.TypeChar, tar.TypeBlock:
			if err := requireParentDir(target); err != nil {
				return fmt.Errorf("failed to extract device %v: %w", header.Name, err)
			}
			mode := uint32(header.Mode)
			if header.Typeflag == tar.TypeChar {
				mode |= syscall.S_IFCHR
			} else {
				mode |= syscall.S_IFBLK
			}
			dev := int(unix.Mkdev(uint32(header.Devmajor), uint32(header.Devminor)))
			if err := syscall.Mknod(target, mode, dev); err != nil {
				return fmt.Errorf("failed to create device %v: %w", target, err)
			}
		case tar.TypeFifo:
			if err := requireParentDir(target); err != nil {
				return fmt.Errorf("failed to extract fifo %v: %w", header.Name, err)
			}
			if err := syscall.Mkfifo(target, uint32(header.Mode)); err != nil {
				return fmt.Errorf("failed to create fifo %v: %w", target, err)
			}
		default:
			return fmt.Errorf("unsupported tar entry type %v for %v", header.Typeflag, header.Name)
		}

		if err := restoreArchiveMetadata(target, header); err != nil {
			return err
		}
	}

	// Restore directories after all entries are extracted. Archive order only
	// guarantees parents before children, not depth-first traversal, so a later
	// child entry may still update an earlier directory's metadata.
	for _, dir := range slices.Backward(dirs) {
		if err := restoreArchiveMetadata(dir.target, &dir.header); err != nil {
			return err
		}
		if err := os.Chmod(dir.target, os.FileMode(dir.header.Mode)); err != nil {
			return fmt.Errorf("failed to restore mode for directory %v: %w", dir.target, err)
		}
	}

	return nil
}

type dirMetadata struct {
	target string
	header tar.Header
}

func restoreArchiveMetadata(target string, header *tar.Header) error {
	if err := os.Lchown(target, header.Uid, header.Gid); err != nil {
		return fmt.Errorf("failed to restore ownership for %v: %w", target, err)
	}

	// os.Chtimes follows symlinks.
	if header.Typeflag != tar.TypeSymlink {
		if err := os.Chtimes(target, header.AccessTime, header.ModTime); err != nil {
			util.Warn("failed to restore timestamps for %v: %v", target, err)
		}
	}

	for key, val := range header.PAXRecords {
		if attrName, ok := strings.CutPrefix(key, "SCHILY.xattr."); ok {
			if err := xattr.LSet(target, attrName, []byte(val)); err != nil {
				util.Warn("failed to restore xattr %v for %v: %v", attrName, target, err)
			}
		}
	}

	return nil
}

func archiveTargetPath(destDir, name string) (string, error) {
	if name == "" || filepath.IsAbs(name) {
		return "", fmt.Errorf("unsafe archive path %v", name)
	}

	clean := filepath.Clean(name)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("unsafe archive path %v", name)
	}

	return filepath.Join(destDir, clean), nil
}

func requireParentDir(path string) error {
	return util.RequireRealDirectory(filepath.Dir(path))
}
