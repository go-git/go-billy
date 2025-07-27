package embedfs

import (
	"embed"
	"testing"

	"github.com/go-git/go-billy/v6/embedfs/internal/testdata"
)

var emptyEmbedFS embed.FS

func TestDebugErrors(t *testing.T) {
	// Test 1: Empty embed.FS with empty path
	emptyFS := New(&emptyEmbedFS)
	_, err1 := emptyFS.ReadDir("")
	t.Logf("Empty FS, empty path: %v", err1)
	
	// Test 2: Empty embed.FS with root path
	_, err2 := emptyFS.ReadDir("/")
	t.Logf("Empty FS, root path: %v", err2)
	
	// Test 3: Non-empty embed.FS with empty path
	richFS := New(testdata.GetTestData())
	_, err3 := richFS.ReadDir("")
	t.Logf("Rich FS, empty path: %v", err3)
	
	// Test 4: Non-empty embed.FS with root path
	entries, err4 := richFS.ReadDir("/")
	t.Logf("Rich FS, root path: %d entries, err: %v", len(entries), err4)
}
