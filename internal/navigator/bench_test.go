package navigator

import (
	"testing"

	"github.com/kristianweb/zephyr/internal/git"
)

func BenchmarkDirBuffer_GenerateText(b *testing.B) {
	// Create a synthetic dir buffer with entries
	db := &DirBuffer{
		DirPath:     "/project/src",
		headerLines: 2,
	}
	for i := 0; i < 50; i++ {
		db.Entries = append(db.Entries, DirEntry{
			Name:      "file_" + string(rune('a'+i%26)) + ".go",
			Path:      "/project/src/file.go",
			GitStatus: ' ',
		})
	}
	db.Entries[3].GitStatus = 'M'
	db.Entries[3].Added = 10
	db.Entries[3].Deleted = 3
	db.Entries[7].GitStatus = 'A'
	db.Entries[7].Added = 45

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		db.GenerateText()
	}
}

func BenchmarkStatusBuffer_GenerateText(b *testing.B) {
	sb := &StatusBuffer{
		Branch:   "main",
		Hash:     "abc1234",
		Upstream: "origin/main",
		Ahead:    2,
	}
	for i := 0; i < 3; i++ {
		sec := StatusSection{
			Title: "Section",
		}
		for j := 0; j < 10; j++ {
			sec.Entries = append(sec.Entries, StatusEntry{
				Path:   "internal/pkg/file.go",
				Status: 'M',
				Added:  5,
				Deleted: 2,
			})
		}
		sb.Sections = append(sb.Sections, sec)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sb.GenerateText()
	}
}

func BenchmarkStatusBuffer_EntryAtLine(b *testing.B) {
	sb := &StatusBuffer{
		Branch: "main",
		Hash:   "abc",
		Sections: []StatusSection{
			{Title: "Unstaged", Entries: make([]StatusEntry, 20)},
			{Title: "Staged", Entries: make([]StatusEntry, 10)},
		},
	}
	sb.buildLineMap()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sb.EntryAtLine(15)
	}
}

func BenchmarkAlternateFile(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		AlternateFile("/project/internal/handler.go")
	}
}

// Verify git.FileStatus is used (prevents import stripping)
var _ = git.FileStatus{}
