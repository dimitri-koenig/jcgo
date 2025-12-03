package parser

import (
	"bufio"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type GitLogEntry struct {
	Commit         string            `json:"commit"`
	Merge          string            `json:"merge,omitempty"`
	AuthorName     *Optional[string] `json:"author,omitempty"`
	AuthorEmail    *Optional[string] `json:"author_email,omitempty"`
	Date           string            `json:"date,omitempty"`
	CommitterName  *Optional[string] `json:"commit_by,omitempty"`
	CommitterEmail *Optional[string] `json:"commit_by_email,omitempty"`
	CommitterDate  string            `json:"commit_by_date,omitempty"`
	Message        string            `json:"message"`
	Epoch          *Optional[int64]  `json:"epoch,omitempty"`
	EpochUtc       *Optional[int64]  `json:"epoch_utc,omitempty"`
	Stats          *GitLogEntryStats `json:"stats,omitempty"`
	Tree           string            `json:"tree,omitempty"`
	Parent         []string          `json:"parent,omitempty"`
	Refs           []string          `json:"refs,omitempty"`
}

type GitLogEntryStats struct {
	FilesChanged uint64                `json:"files_changed"`
	Insertions   uint64                `json:"insertions"`
	Deletions    uint64                `json:"deletions"`
	Files        []string              `json:"files,omitempty"`
	FileStats    []GitLogEntryFileStat `json:"file_stats,omitempty"`
}

type GitLogEntryFileStat struct {
	Name         string  `json:"name"`
	LinesChanged *uint64 `json:"lines_changed"`
}

type Optional[T any] struct {
	Value  T
	IsSet  bool // false → omit, true + Value == 0 is ambiguous → avoid 0 if possible
	IsNull bool // set this if you really need explicit null
}

func (o Optional[T]) MarshalJSON() ([]byte, error) {
	if o.IsNull {
		return []byte("null"), nil
	}
	return json.Marshal(o.Value)
}

// GitLog implements Parser for git log
type GitLog struct{ baseParser }

func (GitLog) Parse(input []byte) (any, error) {
	return parseGitLog(input)
}

// parseGitLog – exact same logic as before, now private
func parseGitLog(data []byte) ([]GitLogEntry, error) {
	var entries []GitLogEntry
	var current *GitLogEntry
	var bodyLines []string

	commitHashRegex := regexp.MustCompile(`^[0-9a-f]{40}$`)

	sc := bufio.NewScanner(strings.NewReader(string(data)))
	for sc.Scan() {
		line := sc.Text()
		parts := strings.Fields(strings.TrimSpace(line))

		if len(parts) > 1 && commitHashRegex.MatchString(parts[0]) {
			if current != nil {
				current.Message = strings.TrimSpace(strings.Join(bodyLines, "\n"))
				entries = append(entries, *current)
				bodyLines = nil
			}

			current = &GitLogEntry{}
			current.Commit = parts[0]
			bodyLines = append(bodyLines, strings.Join(parts[1:], " "))
		}

		countStartingSpacesOnLine := 0
		for _, ch := range sc.Text() {
			if ch == ' ' {
				countStartingSpacesOnLine++
			} else {
				break
			}
		}

		if strings.HasPrefix(line, "commit ") {
			if current != nil {
				current.Message = strings.TrimSpace(strings.Join(bodyLines, "\n"))
				entries = append(entries, *current)
				bodyLines = nil
			}
			current = &GitLogEntry{}
			current.Commit = parts[1]

			if len(parts) > 2 {
				rest := strings.Join(parts[2:], " ")
				if strings.HasPrefix(rest, "(") {
					rest = strings.Trim(rest, "()")
					refs := strings.Split(rest, ",")
					for i := range refs {
						refs[i] = strings.TrimSpace(refs[i])
					}
					// current.Refs = refs
				}
			}
			continue
		}
		if current == nil {
			continue
		}

		switch {
		case strings.HasPrefix(line, "tree "):
			// current.Tree = strings.TrimSpace(strings.TrimPrefix(line, "tree "))
		case strings.HasPrefix(line, "parent "):
			// current.Parent = append(current.Parent, strings.TrimSpace(strings.TrimPrefix(line, "parent ")))
		case strings.HasPrefix(line, "Merge: "):
			current.Merge = strings.TrimPrefix(line, "Merge: ")
		case strings.HasPrefix(line, "Author: "):
			name, email, _ := parseNameEmail(strings.TrimPrefix(line, "Author: "))
			current.AuthorName = &Optional[string]{Value: name, IsSet: true, IsNull: name == ""}
			current.AuthorEmail = &Optional[string]{Value: email, IsSet: true, IsNull: email == ""}

			//current.CommitterName = &current.AuthorName

			if email != "" {
				//current.CommitterEmail = &current.AuthorEmail
			}
		case strings.HasPrefix(line, "AuthorDate: ") || strings.HasPrefix(line, "Date: "):
			if strings.HasPrefix(line, "AuthorDate: ") {
				current.Date = strings.TrimPrefix(line, "AuthorDate: ")
			} else {
				current.Date = strings.TrimPrefix(line, "Date: ")
			}

			current.Date = strings.TrimSpace(current.Date)

			timeWithTimezone := parseGitDate(current.Date)
			timeWithoutTimezone := parseGitDateWithoutTimezone(current.Date)

			if !timeWithTimezone.IsZero() && !timeWithoutTimezone.IsZero() {
				loc, _ := time.LoadLocation("America/Los_Angeles")
				_, offsetSeconds := timeWithTimezone.In(loc).Zone()

				finalUnix := timeWithoutTimezone.Unix() - int64(offsetSeconds)

				current.Epoch = &Optional[int64]{Value: finalUnix}
				current.EpochUtc = &Optional[int64]{Value: timeWithTimezone.Unix(), IsNull: timeWithTimezone.Unix() != timeWithoutTimezone.Unix()}
			}
		case strings.HasPrefix(line, "Commit: "):
			name, email, _ := parseNameEmail(strings.TrimPrefix(line, "Commit: "))
			current.CommitterName = &Optional[string]{Value: name, IsSet: true, IsNull: name == ""}
			current.CommitterEmail = &Optional[string]{Value: email, IsSet: true, IsNull: email == ""}
		case strings.HasPrefix(line, "CommitDate: "):
			current.CommitterDate = strings.TrimPrefix(line, "CommitDate: ")
		case countStartingSpacesOnLine == 1 && strings.Contains(line, " | "):
			if current.Stats == nil {
				current.Stats = &GitLogEntryStats{
					FilesChanged: 0,
					Insertions:   0,
					Deletions:    0,
					Files:        []string{},
					FileStats:    []GitLogEntryFileStat{},
				}
			}

			parts := strings.SplitN(line, " | ", 2)
			current.Stats.Files = append(current.Stats.Files, strings.TrimSpace(parts[0]))

			if len(parts) > 1 {
				changePart := strings.Trim(parts[1], " +-")
				linesChanged := mayParseUint(changePart)

				current.Stats.FileStats = append(current.Stats.FileStats, GitLogEntryFileStat{
					Name:         strings.TrimSpace(parts[0]),
					LinesChanged: linesChanged,
				})
			}
		case countStartingSpacesOnLine == 1 && strings.Contains(line, "file") && strings.Contains(line, "changed") && (strings.Contains(line, "insertion") || strings.Contains(line, "deletion")):
			if current.Stats == nil {
				current.Stats = &GitLogEntryStats{
					FilesChanged: 0,
					Insertions:   0,
					Deletions:    0,
				}
			}

			filesChanged := parts[0]
			current.Stats.FilesChanged = mustParseUint(filesChanged)

			// go through parts and find insertions and deletions
			for i := 1; i < len(parts); i++ {
				if strings.Contains(parts[i], "insertion") {
					insertions := parts[i-1]
					current.Stats.Insertions = mustParseUint(insertions)
				} else if strings.Contains(parts[i], "deletion") {
					deletions := parts[i-1]
					current.Stats.Deletions = mustParseUint(deletions)
				}
			}
		case countStartingSpacesOnLine == 4:
			bodyLines = append(bodyLines, strings.TrimSpace(line))
		}
	}

	if current != nil {
		current.Message = strings.TrimSpace(strings.Join(bodyLines, "\n"))
		entries = append(entries, *current)
	}

	return entries, sc.Err()
}

func parseNameEmail(s string) (name, email string, ok bool) {
	s = strings.TrimSpace(s)
	if i := strings.LastIndex(s, "<"); i >= 0 {
		name = strings.TrimSpace(s[:i])
		email = strings.Trim(s[i+1:], " <>")
		return name, email, true
	}
	return s, "", false
}

func parseGitDate(s string) time.Time {
	s = strings.TrimSpace(s)
	if len(s) < 24 {
		return time.Time{}
	}

	t, _ := time.Parse("Mon Jan 2 15:04:05 2006 -0700", s)

	if t.IsZero() {
		return time.Time{}
	}
	return t
}

func parseGitDateWithoutTimezone(s string) time.Time {
	s = strings.TrimSpace(s)
	if len(s) < 24 {
		return time.Time{}
	}

	s = s[:len(s)-6] + " +0000" // Normalize timezone for parsing

	t, _ := time.Parse("Mon Jan 2 15:04:05 2006 -0700", s)

	if t.IsZero() {
		return time.Time{}
	}
	return t
}

func mustParseUint(s string) uint64 {
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		// return nil on error
		return 0
	}
	return v
}

func mayParseUint(s string) *uint64 {
	intValue, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		// return nil on error
		return nil
	}
	return &intValue
}
