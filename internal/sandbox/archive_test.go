package sandbox

import (
	"archive/tar"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"
)

func TestArchive(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "src")
	destDir := filepath.Join(tmpDir, "dest")
	archiveFile := filepath.Join(tmpDir, "archive.tar.zst")

	// 1. Prepare source directory
	os.MkdirAll(filepath.Join(srcDir, "subdir"), 0755)
	os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("hello"), 0644)
	os.WriteFile(filepath.Join(srcDir, "subdir/file2.txt"), []byte("world"), 0644)
	os.Symlink("file1.txt", filepath.Join(srcDir, "link1"))

	// 2. Compress
	err := Compress(srcDir, archiveFile, zstd.SpeedDefault)
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}

	// 3. Extract
	err = Extract(archiveFile, destDir)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	// 5. Verify extracted content
	// Note: Compress archives the *contents* of srcDir, so it extracts directly to destDir/...

	content1, err := os.ReadFile(filepath.Join(destDir, "file1.txt"))
	if err != nil {
		t.Errorf("failed to read file1.txt: %v", err)
	} else if string(content1) != "hello" {
		t.Errorf("file1.txt content mismatch: got %q, want %q", string(content1), "hello")
	}

	content2, err := os.ReadFile(filepath.Join(destDir, "subdir/file2.txt"))
	if err != nil {
		t.Errorf("failed to read subdir/file2.txt: %v", err)
	} else if string(content2) != "world" {
		t.Errorf("file2.txt content mismatch: got %q, want %q", string(content2), "world")
	}

	linkTarget, err := os.Readlink(filepath.Join(destDir, "link1"))
	if err != nil {
		t.Errorf("failed to read link1: %v", err)
	} else if linkTarget != "file1.txt" {
		t.Errorf("link1 target mismatch: got %q, want %q", linkTarget, "file1.txt")
	}

	// Verify directory exists
	if info, err := os.Stat(filepath.Join(destDir, "subdir")); err != nil || !info.IsDir() {
		t.Errorf("subdir missing or not a directory")
	}
}

func TestArchiveHardLink(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "src")
	destDir := filepath.Join(tmpDir, "dest")
	archiveFile := filepath.Join(tmpDir, "archive.tar.zst")

	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "original"), []byte("data"), 0644)
	err := os.Link(filepath.Join(srcDir, "original"), filepath.Join(srcDir, "hardlink"))
	if err != nil {
		t.Skipf("Hard links not supported on this filesystem: %v", err)
	}

	err = Compress(srcDir, archiveFile, zstd.SpeedDefault)
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}

	err = Extract(archiveFile, destDir)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	fi1, err := os.Stat(filepath.Join(destDir, "original"))
	if err != nil {
		t.Fatalf("original missing: %v", err)
	}
	fi2, err := os.Stat(filepath.Join(destDir, "hardlink"))
	if err != nil {
		t.Fatalf("hardlink missing: %v", err)
	}

	// Verify they are the same file (same inode)
	if !reflect.DeepEqual(fi1.Sys(), fi2.Sys()) {
		content1, _ := os.ReadFile(filepath.Join(destDir, "original"))
		content2, _ := os.ReadFile(filepath.Join(destDir, "hardlink"))
		if string(content1) != string(content2) {
			t.Errorf("Hard link content mismatch")
		}
	}
}

func TestCompressReportsFinalizationError(t *testing.T) {
	if _, err := os.Stat("/dev/full"); err != nil {
		t.Skipf("/dev/full is not available: %v", err)
	}

	err := Compress(t.TempDir(), "/dev/full", zstd.SpeedDefault)
	if err == nil {
		t.Fatal("Compress ignored a finalization error")
	}
}

func TestExtractRejectsParentTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	destDir := filepath.Join(tmpDir, "dest")
	archiveFile := filepath.Join(tmpDir, "archive.tar.zst")
	escapedPath := filepath.Join(tmpDir, "escaped")

	writeTestArchive(t, archiveFile, []testArchiveEntry{{
		name: "../escaped",
		body: "escaped",
		mode: 0o644,
	}})

	if err := Extract(archiveFile, destDir); err == nil {
		t.Fatal("Extract accepted a parent traversal path")
	}
	if _, err := os.Stat(escapedPath); !os.IsNotExist(err) {
		t.Fatalf("escaped path exists after failed extraction: %v", err)
	}
}

func TestExtractRejectsAbsolutePath(t *testing.T) {
	tmpDir := t.TempDir()
	destDir := filepath.Join(tmpDir, "dest")
	archiveFile := filepath.Join(tmpDir, "archive.tar.zst")

	writeTestArchive(t, archiveFile, []testArchiveEntry{{
		name: "/tmp/opqu-sandbox-absolute-path-test",
		body: "absolute",
		mode: 0o644,
	}})

	if err := Extract(archiveFile, destDir); err == nil {
		t.Fatal("Extract accepted an absolute path")
	}
}

func TestExtractRejectsExistingDestination(t *testing.T) {
	tmpDir := t.TempDir()
	destDir := filepath.Join(tmpDir, "dest")
	archiveFile := filepath.Join(tmpDir, "archive.tar.zst")

	if err := os.Mkdir(destDir, 0o755); err != nil {
		t.Fatalf("failed to create destination directory: %v", err)
	}
	writeTestArchive(t, archiveFile, []testArchiveEntry{{
		name: "file",
		body: "data",
		mode: 0o644,
	}})

	if err := Extract(archiveFile, destDir); err == nil {
		t.Fatal("Extract accepted an existing destination")
	}
}

func TestExtractRejectsEscapingHardLink(t *testing.T) {
	tmpDir := t.TempDir()
	destDir := filepath.Join(tmpDir, "dest")
	archiveFile := filepath.Join(tmpDir, "archive.tar.zst")

	writeTestArchive(t, archiveFile, []testArchiveEntry{
		{
			name: "file",
			body: "data",
			mode: 0o644,
		},
		{
			name:     "link",
			linkname: "../outside",
			typeflag: tar.TypeLink,
			mode:     0o644,
		},
	})

	if err := Extract(archiveFile, destDir); err == nil {
		t.Fatal("Extract accepted a hard link target outside the destination")
	}
}

func TestExtractRejectsFileUnderSymlinkParent(t *testing.T) {
	tmpDir := t.TempDir()
	destDir := filepath.Join(tmpDir, "dest")
	archiveFile := filepath.Join(tmpDir, "archive.tar.zst")
	outsideDir := filepath.Join(tmpDir, "outside")

	if err := os.Mkdir(outsideDir, 0o755); err != nil {
		t.Fatalf("failed to create outside directory: %v", err)
	}

	writeTestArchive(t, archiveFile, []testArchiveEntry{
		{
			name:     "escape",
			linkname: outsideDir,
			typeflag: tar.TypeSymlink,
			mode:     0o777,
		},
		{
			name: "escape/file",
			body: "escaped",
			mode: 0o644,
		},
	})

	if err := Extract(archiveFile, destDir); err == nil {
		t.Fatal("Extract accepted a file under a symlink parent")
	}
	if _, err := os.Stat(filepath.Join(outsideDir, "file")); !os.IsNotExist(err) {
		t.Fatalf("outside file exists after failed extraction: %v", err)
	}
}

func TestExtractRestoresDirectoryMetadataAfterChildren(t *testing.T) {
	tmpDir := t.TempDir()
	destDir := filepath.Join(tmpDir, "dest")
	archiveFile := filepath.Join(tmpDir, "archive.tar.zst")
	dirModTime := time.Unix(946684800, 0)

	writeTestArchive(t, archiveFile, []testArchiveEntry{
		{
			name:     "dir",
			typeflag: tar.TypeDir,
			mode:     0o711,
			modTime:  dirModTime,
		},
		{
			name: "dir/file",
			body: "data",
			mode: 0o644,
		},
	})

	if err := Extract(archiveFile, destDir); err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	info, err := os.Stat(filepath.Join(destDir, "dir"))
	if err != nil {
		t.Fatalf("failed to stat extracted directory: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o711 {
		t.Fatalf("directory mode = %v, want %v", got, os.FileMode(0o711))
	}
	if got := info.ModTime(); !got.Equal(dirModTime) {
		t.Fatalf("directory mtime = %v, want %v", got, dirModTime)
	}
}

func TestExtractRestoresRegularFileModeAfterUmask(t *testing.T) {
	tmpDir := t.TempDir()
	destDir := filepath.Join(tmpDir, "dest")
	archiveFile := filepath.Join(tmpDir, "archive.tar.zst")

	writeTestArchive(t, archiveFile, []testArchiveEntry{{
		name: "file",
		body: "data",
		mode: 0o666,
	}})

	oldUmask := syscall.Umask(0o077)
	defer syscall.Umask(oldUmask)

	if err := Extract(archiveFile, destDir); err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	info, err := os.Stat(filepath.Join(destDir, "file"))
	if err != nil {
		t.Fatalf("failed to stat extracted file: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o666 {
		t.Fatalf("file mode = %v, want %v", got, os.FileMode(0o666))
	}
}

func TestExtractRejectsUnsupportedEntryType(t *testing.T) {
	tmpDir := t.TempDir()
	destDir := filepath.Join(tmpDir, "dest")
	archiveFile := filepath.Join(tmpDir, "archive.tar.zst")

	writeTestArchive(t, archiveFile, []testArchiveEntry{{
		name:     "unsupported",
		typeflag: 'X',
		mode:     0o644,
	}})

	err := Extract(archiveFile, destDir)
	if err == nil {
		t.Fatal("Extract accepted an unsupported entry type")
	}
	if !strings.Contains(err.Error(), "unsupported tar entry type") {
		t.Fatalf("Extract returned %q, want unsupported-entry-type error", err)
	}
}

type testArchiveEntry struct {
	name     string
	body     string
	linkname string
	typeflag byte
	mode     int64
	modTime  time.Time
}

func writeTestArchive(t *testing.T, archiveFile string, entries []testArchiveEntry) {
	t.Helper()

	f, err := os.Create(archiveFile)
	if err != nil {
		t.Fatalf("failed to create archive: %v", err)
	}
	defer f.Close()

	zw, err := zstd.NewWriter(f, zstd.WithEncoderLevel(zstd.SpeedDefault))
	if err != nil {
		t.Fatalf("failed to create zstd writer: %v", err)
	}

	tw := tar.NewWriter(zw)
	for _, entry := range entries {
		typeflag := entry.typeflag
		if typeflag == 0 {
			typeflag = tar.TypeReg
		}

		header := &tar.Header{
			Name:     entry.name,
			Mode:     entry.mode,
			Size:     int64(len(entry.body)),
			Typeflag: typeflag,
			Linkname: entry.linkname,
			ModTime:  entry.modTime,
			Uid:      os.Getuid(),
			Gid:      os.Getgid(),
		}
		if typeflag != tar.TypeReg {
			header.Size = 0
		}

		if err := tw.WriteHeader(header); err != nil {
			t.Fatalf("failed to write tar header: %v", err)
		}
		if typeflag == tar.TypeReg {
			if _, err := tw.Write([]byte(entry.body)); err != nil {
				t.Fatalf("failed to write tar body: %v", err)
			}
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("failed to close tar writer: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("failed to close zstd writer: %v", err)
	}
}
