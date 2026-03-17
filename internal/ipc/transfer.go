// Package ipc provides inter-process communication for transferring tabs
// between Zephyr instances using file-based signaling in the OS temp directory.
package ipc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

const offerFileName = "zephyr-drag-offer.json"

// TabTransfer holds the metadata for a tab being transferred between instances.
type TabTransfer struct {
	ContentFile string `json:"content_file"` // path to temp file with tab content
	Title       string `json:"title"`
	Language    string `json:"language"`
	FilePath    string `json:"file_path"` // original file path, empty for untitled
	Modified    bool   `json:"modified"`
	SourcePID   int    `json:"source_pid"`
	Timestamp   int64  `json:"timestamp"` // unix millis, for staleness detection
}

func offerPath() string {
	return filepath.Join(os.TempDir(), offerFileName)
}

// WriteOffer creates a drag offer file that other instances can detect.
func WriteOffer(t TabTransfer) error {
	t.SourcePID = os.Getpid()
	t.Timestamp = time.Now().UnixMilli()
	data, err := json.Marshal(t)
	if err != nil {
		return err
	}
	return os.WriteFile(offerPath(), data, 0644)
}

// ReadOffer reads the current drag offer if one exists and is fresh (< 5s old).
// Returns nil if no offer exists or it's stale.
func ReadOffer() *TabTransfer {
	data, err := os.ReadFile(offerPath())
	if err != nil {
		return nil
	}
	var t TabTransfer
	if err := json.Unmarshal(data, &t); err != nil {
		return nil
	}
	// Ignore stale offers (> 5 seconds old)
	if time.Now().UnixMilli()-t.Timestamp > 5000 {
		os.Remove(offerPath())
		return nil
	}
	// Ignore offers from our own process
	if t.SourcePID == os.Getpid() {
		return nil
	}
	return &t
}

// ClaimOffer atomically reads and removes the offer file, returning the offer.
// Returns nil if no offer exists or it was already claimed.
func ClaimOffer() *TabTransfer {
	offer := ReadOffer()
	if offer == nil {
		return nil
	}
	// Remove the offer file to signal that we claimed it
	os.Remove(offerPath())
	// Write a claim file so the source knows it was consumed
	claimPath := filepath.Join(os.TempDir(), "zephyr-drag-claimed-"+strconv.Itoa(offer.SourcePID))
	os.WriteFile(claimPath, []byte("claimed"), 0644)
	return offer
}

// WasClaimed checks if our offer was claimed by another instance.
// Removes the claim file if found.
func WasClaimed() bool {
	claimPath := filepath.Join(os.TempDir(), "zephyr-drag-claimed-"+strconv.Itoa(os.Getpid()))
	if _, err := os.Stat(claimPath); err == nil {
		os.Remove(claimPath)
		return true
	}
	return false
}

// CleanupOffer removes any existing offer from this process.
func CleanupOffer() {
	data, err := os.ReadFile(offerPath())
	if err != nil {
		return
	}
	var t TabTransfer
	if err := json.Unmarshal(data, &t); err != nil {
		return
	}
	if t.SourcePID == os.Getpid() {
		os.Remove(offerPath())
	}
}
