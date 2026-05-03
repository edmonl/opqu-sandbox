package sandbox

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

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

	// 3. ListPaths
	paths, err := ListPaths(archiveFile)
	if err != nil {
		t.Fatalf("ListPaths failed: %v", err)
	}
	expectedPaths := []string{"file1.txt", "link1", "subdir", "subdir/file2.txt"}
	// ListPaths output depends on filepath.Walk order, but usually sorted or consistent on same FS
	// Let's just check if all expected paths are present
	pathMap := make(map[string]bool)
	for _, p := range paths {
		pathMap[p] = true
	}
	for _, p := range expectedPaths {
		if !pathMap[p] {
			t.Errorf("Expected path %v missing from ListPaths output: %v", p, paths)
		}
	}

	// 4. Extract
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
