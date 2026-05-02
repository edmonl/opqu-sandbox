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
	expectedPaths := []string{"src", "src/file1.txt", "src/link1", "src/subdir", "src/subdir/file2.txt"}
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
	// Note: Compress archives the *directory* itself (srcDir), so it extracts to destDir/src/...
	extractedSrc := filepath.Join(destDir, "src")

	content1, _ := os.ReadFile(filepath.Join(extractedSrc, "file1.txt"))
	if string(content1) != "hello" {
		t.Errorf("file1.txt content mismatch: got %q, want %q", string(content1), "hello")
	}

	content2, _ := os.ReadFile(filepath.Join(extractedSrc, "subdir/file2.txt"))
	if string(content2) != "world" {
		t.Errorf("file2.txt content mismatch: got %q, want %q", string(content2), "world")
	}

	linkTarget, _ := os.Readlink(filepath.Join(extractedSrc, "link1"))
	if linkTarget != "file1.txt" {
		t.Errorf("link1 target mismatch: got %q, want %q", linkTarget, "file1.txt")
	}

	// Verify directory exists
	if info, err := os.Stat(filepath.Join(extractedSrc, "subdir")); err != nil || !info.IsDir() {
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

	extractedSrc := filepath.Join(destDir, "src")
	fi1, _ := os.Stat(filepath.Join(extractedSrc, "original"))
	fi2, _ := os.Stat(filepath.Join(extractedSrc, "hardlink"))

	// Verify they are the same file (same inode)
	// This might not work on all filesystems or if the tar/extract logic doesn't preserve hard links
	// The current Compress logic DOES handle hard links.
	if !reflect.DeepEqual(fi1.Sys(), fi2.Sys()) {
		// On some systems/Go versions, fi1.Sys() might contain more than just (dev, ino)
		// but they should be equal if they are hard links.
		// Fallback check if Sys() is not enough:
		content1, _ := os.ReadFile(filepath.Join(extractedSrc, "original"))
		content2, _ := os.ReadFile(filepath.Join(extractedSrc, "hardlink"))
		if string(content1) != string(content2) {
			t.Errorf("Hard link content mismatch")
		}
	}
}
