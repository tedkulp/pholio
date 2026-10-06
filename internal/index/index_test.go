package index_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/tedkulp/pholio/internal/index"
	"github.com/tedkulp/pholio/internal/seam"
	"github.com/tedkulp/pholio/internal/testutil"
)

func scanned(t *testing.T, vault string) (*index.Index, string) {
	t.Helper()
	root := testutil.CopyVault(t, vault)
	ix := index.New(seam.OSFS{}, root)
	if err := ix.Scan(context.Background()); err != nil {
		t.Fatalf("Scan: %v", err)
	}
	return ix, root
}

func paths(notes []index.Note) []string {
	var ps []string
	for _, n := range notes {
		ps = append(ps, n.Path)
	}
	return ps
}

func TestScanIndexesMarkdownAndSkipsDotDirsAndOtherFiles(t *testing.T) {
	ix, _ := scanned(t, "links")
	want := []string{
		"a/dup.md",
		"alpha.md",
		"alpha.sync-conflict-20261001-101010-ABCDEFG.md",
		"b/c/dup.md",
		"deep/er/beta.md",
		"index.md",
		"sub/gamma.md",
	}
	if got := paths(ix.Notes()); !reflect.DeepEqual(got, want) {
		t.Errorf("Notes()\n got %v\nwant %v", got, want)
	}
	n, ok := ix.Note("alpha.md")
	if !ok || n.Title != "Alpha" || len(n.Headings) != 2 {
		t.Errorf("Note(alpha.md) = %+v, %v", n, ok)
	}
}

func TestReadyOnlyAfterTheFirstScan(t *testing.T) {
	root := testutil.CopyVault(t, "basic")
	ix := index.New(seam.OSFS{}, root)
	if ix.Ready() {
		t.Fatal("Ready before Scan")
	}
	select {
	case <-ix.Done():
		t.Fatal("Done closed before Scan")
	default:
	}
	if err := ix.Scan(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !ix.Ready() {
		t.Fatal("not Ready after Scan")
	}
	<-ix.Done()
	n, ok := ix.Note("README.md")
	if !ok || n.Title != "Welcome to the basic Vault" || len(n.Tasks) != 1 || len(n.Links) != 1 {
		t.Errorf("README.md = %+v", n)
	}
}

func TestScanFailsWhenTheVaultIsMissing(t *testing.T) {
	ix := index.New(seam.OSFS{}, t.TempDir()+"/nope")
	if err := ix.Scan(context.Background()); err == nil {
		t.Fatal("Scan of missing Vault succeeded")
	}
	if ix.Ready() {
		t.Error("Ready after failed Scan")
	}
}
