package ipc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// isolate points the transfer files at a private directory for one test, the
// way the GUI harness points a test instance away from a developer's Zephyr.
func isolate(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("ZEPHYR_GUI_STATE_DIR", dir)
	return dir
}

// writeForeignOffer plants an offer that looks like it came from another
// process, which is the only kind ReadOffer will hand back.
func writeForeignOffer(t *testing.T, offer TabTransfer) {
	t.Helper()
	if offer.SourcePID == 0 {
		offer.SourcePID = os.Getpid() + 1
	}
	if offer.Timestamp == 0 {
		offer.Timestamp = time.Now().UnixMilli()
	}
	data, err := json.Marshal(offer)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(offerPath(), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestOfferPath_HonoursStateDirOverride(t *testing.T) {
	dir := isolate(t)
	if got, want := offerPath(), filepath.Join(dir, offerFileName); got != want {
		t.Fatalf("offerPath() = %q, want %q", got, want)
	}
	if got, want := claimPath(42), filepath.Join(dir, claimFilePrefix+"42"); got != want {
		t.Fatalf("claimPath(42) = %q, want %q", got, want)
	}

	if err := WriteOffer(TabTransfer{Title: "scratch"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, offerFileName)); err != nil {
		t.Fatalf("offer was not written into the state dir: %v", err)
	}
	// The whole point of the override: a Zephyr without it must not see this
	// offer, so it can never claim a test instance's tab.
	t.Setenv("ZEPHYR_GUI_STATE_DIR", t.TempDir())
	if offer := ReadOffer(); offer != nil {
		t.Fatalf("an offer in another state dir was visible: %+v", offer)
	}
}

func TestOfferPath_DefaultsToTempDir(t *testing.T) {
	t.Setenv("ZEPHYR_GUI_STATE_DIR", "")
	if got, want := offerPath(), filepath.Join(os.TempDir(), offerFileName); got != want {
		t.Fatalf("offerPath() = %q, want %q", got, want)
	}
}

func TestWriteReadClaim_RoundTrip(t *testing.T) {
	isolate(t)
	want := TabTransfer{
		ContentFile: "/tmp/zephyr-tab-content",
		Title:       "notes.md",
		Language:    "Markdown",
		FilePath:    "/Users/someone/notes.md",
		Modified:    true,
	}
	writeForeignOffer(t, want)

	got := ReadOffer()
	if got == nil {
		t.Fatal("ReadOffer returned nil for a fresh offer from another process")
	}
	if got.ContentFile != want.ContentFile || got.Title != want.Title ||
		got.Language != want.Language || got.FilePath != want.FilePath ||
		got.Modified != want.Modified {
		t.Fatalf("ReadOffer = %+v, want the fields of %+v", *got, want)
	}
	// Reading is not claiming: the offer has to survive for the instance the
	// pointer is actually over.
	if ReadOffer() == nil {
		t.Fatal("ReadOffer consumed the offer")
	}

	claimed := ClaimOffer()
	if claimed == nil || claimed.Title != want.Title {
		t.Fatalf("ClaimOffer = %+v, want the offer", claimed)
	}
	if ReadOffer() != nil {
		t.Fatal("the offer survived being claimed")
	}
	if ClaimOffer() != nil {
		t.Fatal("a second claim succeeded")
	}
	if _, err := os.Stat(claimPath(got.SourcePID)); err != nil {
		t.Fatalf("no claim receipt was left for the source pid: %v", err)
	}
}

func TestReadOffer_IgnoresOurOwnOffer(t *testing.T) {
	isolate(t)
	if err := WriteOffer(TabTransfer{Title: "mine"}); err != nil {
		t.Fatal(err)
	}
	if offer := ReadOffer(); offer != nil {
		t.Fatalf("ReadOffer returned our own offer: %+v", offer)
	}
}

func TestReadOffer_DropsStaleOffer(t *testing.T) {
	isolate(t)
	writeForeignOffer(t, TabTransfer{
		Title:     "stale",
		Timestamp: time.Now().Add(-6 * time.Second).UnixMilli(),
	})
	if offer := ReadOffer(); offer != nil {
		t.Fatalf("a six-second-old offer was accepted: %+v", offer)
	}
	if _, err := os.Stat(offerPath()); !os.IsNotExist(err) {
		t.Fatalf("the stale offer file was left behind: %v", err)
	}
}

func TestReadOffer_NoOfferAndCorruptOffer(t *testing.T) {
	isolate(t)
	if offer := ReadOffer(); offer != nil {
		t.Fatalf("ReadOffer invented an offer: %+v", offer)
	}
	if err := os.WriteFile(offerPath(), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if offer := ReadOffer(); offer != nil {
		t.Fatalf("a corrupt offer file was accepted: %+v", offer)
	}
}

func TestWasClaimed_ConsumesTheReceipt(t *testing.T) {
	isolate(t)
	if WasClaimed() {
		t.Fatal("WasClaimed reported a claim with no receipt on disk")
	}
	if err := os.WriteFile(claimPath(os.Getpid()), []byte("claimed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !WasClaimed() {
		t.Fatal("WasClaimed missed our receipt")
	}
	if WasClaimed() {
		t.Fatal("WasClaimed reported the same claim twice")
	}
}

func TestCleanupOffer_RemovesOnlyOurOwn(t *testing.T) {
	isolate(t)
	if err := WriteOffer(TabTransfer{Title: "mine"}); err != nil {
		t.Fatal(err)
	}
	CleanupOffer()
	if _, err := os.Stat(offerPath()); !os.IsNotExist(err) {
		t.Fatalf("CleanupOffer left our own offer behind: %v", err)
	}

	writeForeignOffer(t, TabTransfer{Title: "theirs"})
	CleanupOffer()
	if _, err := os.Stat(offerPath()); err != nil {
		t.Fatalf("CleanupOffer removed another process's offer: %v", err)
	}
}
