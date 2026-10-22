package domain

import "time"

// PipelineStats — снимок воронки для live-панели и дашборда команды.
type PipelineStats struct {
	NewsByStatus    map[PipelineStatus]int
	StoriesByStatus map[PipelineStatus]int
	ResearchDocs    int
	ResearchChunks  int
	NewsLast24h     int
	LastFetchedAt   *time.Time
}
