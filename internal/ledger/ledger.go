package ledger

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// DefaultDir returns the default ledger directory (~/.alloy/ledgers).
func DefaultDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}
	return filepath.Join(home, ".alloy", "ledgers"), nil
}

// DefaultBackupDir returns the default backup directory (~/.alloy/backups).
func DefaultBackupDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}
	return filepath.Join(home, ".alloy", "backups"), nil
}

// Ledger tracks file system operations for a single package installation.
type Ledger struct {
	// Header contains metadata about this ledger.
	Header Header

	// Entries contains all recorded operations in chronological order.
	Entries []Entry

	// path is the file path where this ledger is persisted.
	path string

	// file is the open file handle for appending entries.
	file *os.File
}

// Path returns the file path for a package's ledger.
func Path(dir, pkg string) string {
	return filepath.Join(dir, pkg+".jsonl")
}

// Create creates a new ledger for a package installation.
// The ledger file is created immediately and the header is written.
func Create(dir, pkg, source string) (*Ledger, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create ledger directory: %w", err)
	}

	path := Path(dir, pkg)

	// Check if ledger already exists
	if _, err := os.Stat(path); err == nil {
		return nil, fmt.Errorf("ledger already exists for package %q", pkg)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0644)
	if err != nil {
		return nil, fmt.Errorf("create ledger file: %w", err)
	}

	header := Header{
		Version:     CurrentVersion,
		Package:     pkg,
		InstalledAt: time.Now().UTC(),
		Source:      source,
	}

	l := &Ledger{
		Header: header,
		path:   path,
		file:   f,
	}

	// Write header as first line
	if err := l.writeJSON(header); err != nil {
		f.Close()
		os.Remove(path)
		return nil, fmt.Errorf("write header: %w", err)
	}

	return l, nil
}

// Open opens an existing ledger for reading.
// The entire ledger is loaded into memory.
func Open(dir, pkg string) (*Ledger, error) {
	path := Path(dir, pkg)
	return OpenPath(path)
}

// OpenPath opens a ledger from a specific file path.
func OpenPath(path string) (*Ledger, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open ledger file: %w", err)
	}
	defer f.Close()

	l := &Ledger{path: path}

	scanner := bufio.NewScanner(f)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()

		if lineNum == 1 {
			// First line is the header
			if err := json.Unmarshal(line, &l.Header); err != nil {
				return nil, fmt.Errorf("parse header (line 1): %w", err)
			}
			if l.Header.Version > CurrentVersion {
				return nil, fmt.Errorf("ledger version %d is newer than supported version %d",
					l.Header.Version, CurrentVersion)
			}
			continue
		}

		var entry Entry
		if err := json.Unmarshal(line, &entry); err != nil {
			return nil, fmt.Errorf("parse entry (line %d): %w", lineNum, err)
		}
		l.Entries = append(l.Entries, entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read ledger: %w", err)
	}

	if lineNum == 0 {
		return nil, errors.New("ledger file is empty")
	}

	return l, nil
}

// Append opens an existing ledger for appending new entries.
func Append(dir, pkg string) (*Ledger, error) {
	path := Path(dir, pkg)

	// First, read the existing ledger
	l, err := OpenPath(path)
	if err != nil {
		return nil, err
	}

	// Open for appending
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("open ledger for append: %w", err)
	}

	l.file = f
	return l, nil
}

// Record writes a new entry to the ledger.
// The entry is immediately persisted to disk.
func (l *Ledger) Record(entry Entry) error {
	if l.file == nil {
		return errors.New("ledger not open for writing")
	}

	// Ensure timestamp is set
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}

	if err := l.writeJSON(entry); err != nil {
		return fmt.Errorf("write entry: %w", err)
	}

	l.Entries = append(l.Entries, entry)
	return nil
}

// Close closes the ledger file.
func (l *Ledger) Close() error {
	if l.file != nil {
		if err := l.file.Sync(); err != nil {
			l.file.Close()
			return fmt.Errorf("sync ledger: %w", err)
		}
		if err := l.file.Close(); err != nil {
			return fmt.Errorf("close ledger: %w", err)
		}
		l.file = nil
	}
	return nil
}

// Delete removes the ledger file from disk.
func (l *Ledger) Delete() error {
	if l.file != nil {
		l.file.Close()
		l.file = nil
	}
	return os.Remove(l.path)
}

// writeJSON writes a value as a single JSON line.
func (l *Ledger) writeJSON(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = l.file.Write(data)
	return err
}

// List returns the names of all packages with ledgers in the directory.
func List(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read ledger directory: %w", err)
	}

	var packages []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if ext := filepath.Ext(name); ext == ".jsonl" {
			packages = append(packages, name[:len(name)-len(ext)])
		}
	}
	return packages, nil
}

// Exists checks if a ledger exists for the given package.
func Exists(dir, pkg string) bool {
	_, err := os.Stat(Path(dir, pkg))
	return err == nil
}

// Stream reads ledger entries one at a time without loading all into memory.
// Useful for large ledgers or when processing entries sequentially.
type Stream struct {
	file    *os.File
	scanner *bufio.Scanner
	header  Header
	lineNum int
	err     error
}

// OpenStream opens a ledger for streaming reads.
func OpenStream(dir, pkg string) (*Stream, error) {
	path := Path(dir, pkg)
	return OpenStreamPath(path)
}

// OpenStreamPath opens a ledger stream from a file path.
func OpenStreamPath(path string) (*Stream, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open ledger file: %w", err)
	}

	s := &Stream{
		file:    f,
		scanner: bufio.NewScanner(f),
	}

	// Read header
	if !s.scanner.Scan() {
		f.Close()
		if err := s.scanner.Err(); err != nil {
			return nil, fmt.Errorf("read header: %w", err)
		}
		return nil, errors.New("ledger file is empty")
	}

	if err := json.Unmarshal(s.scanner.Bytes(), &s.header); err != nil {
		f.Close()
		return nil, fmt.Errorf("parse header: %w", err)
	}

	s.lineNum = 1
	return s, nil
}

// Header returns the ledger header.
func (s *Stream) Header() Header {
	return s.header
}

// Next reads the next entry. Returns io.EOF when done.
func (s *Stream) Next() (Entry, error) {
	if s.err != nil {
		return Entry{}, s.err
	}

	if !s.scanner.Scan() {
		if err := s.scanner.Err(); err != nil {
			s.err = err
			return Entry{}, err
		}
		s.err = io.EOF
		return Entry{}, io.EOF
	}

	s.lineNum++
	var entry Entry
	if err := json.Unmarshal(s.scanner.Bytes(), &entry); err != nil {
		s.err = fmt.Errorf("parse entry (line %d): %w", s.lineNum, err)
		return Entry{}, s.err
	}

	return entry, nil
}

// Close closes the stream.
func (s *Stream) Close() error {
	return s.file.Close()
}
