package main

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Note is one Markdown file of the vault.
type Note struct {
	Path    string // vault-relative, slash separated, with extension
	Title   string // file name without .md
	Dir     string // vault-relative folder ("" for root)
	Tags    []string
	Date    string // frontmatter "fecha" or "actualizado", if any
	ModTime time.Time
	Body    string   // Markdown without frontmatter
	Links   []string // resolved target paths (unique)
	search  string   // folded title + body, for search
}

// Vault is an in-memory index of the synced vault folder.
type Vault struct {
	mu        sync.RWMutex
	root      string
	notes     map[string]*Note    // by path
	byName    map[string][]string // folded base name -> paths
	byPath    map[string]string   // folded path without .md -> path
	backlinks map[string][]string // target path -> source paths
	tags      map[string][]string // tag -> paths
	files     map[string]bool     // every non-hidden file (incl. non-md)
	dirs      map[string]bool     // every folder ("" = root)
	indexedAt time.Time
}

func NewVault(root string) *Vault { return &Vault{root: root} }

var wikiLinkRe = regexp.MustCompile(`!?\[\[([^\[\]|#\\]+)(#[^\[\]|\\]*)?(?:\\?\|([^\[\]]+))?\]\]`)

// fold lower-cases and strips accents, for matching names and search.
func fold(s string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(s) {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

func hidden(rel string) bool {
	for _, part := range strings.Split(rel, "/") {
		if strings.HasPrefix(part, ".") {
			return true
		}
	}
	return false
}

// Reindex rescans the vault folder from disk.
func (v *Vault) Reindex() error {
	notes := map[string]*Note{}
	files := map[string]bool{}
	dirs := map[string]bool{"": true}
	err := filepath.WalkDir(v.root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(v.root, p)
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if hidden(rel) || strings.HasSuffix(rel, ".sdr") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			dirs[rel] = true
			return nil
		}
		files[rel] = true
		if !strings.HasSuffix(strings.ToLower(rel), ".md") {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		info, _ := d.Info()
		n := &Note{Path: rel, Dir: path.Dir(rel), Title: strings.TrimSuffix(path.Base(rel), path.Ext(rel))}
		if n.Dir == "." {
			n.Dir = ""
		}
		if info != nil {
			n.ModTime = info.ModTime()
		}
		n.Body, n.Tags, n.Date = splitFrontmatter(string(data))
		n.search = fold(n.Title + "\n" + n.Body)
		notes[rel] = n
		return nil
	})
	if err != nil {
		return err
	}

	byName := map[string][]string{}
	byPath := map[string]string{}
	tags := map[string][]string{}
	for p, n := range notes {
		byName[fold(n.Title)] = append(byName[fold(n.Title)], p)
		byPath[fold(strings.TrimSuffix(p, path.Ext(p)))] = p
		for _, t := range n.Tags {
			tags[t] = append(tags[t], p)
		}
	}
	for _, list := range byName {
		sort.Strings(list)
	}

	nv := &Vault{root: v.root, notes: notes, byName: byName, byPath: byPath, tags: tags, files: files, dirs: dirs}
	backlinks := map[string][]string{}
	for p, n := range notes {
		seen := map[string]bool{}
		for _, m := range wikiLinkRe.FindAllStringSubmatch(n.Body, -1) {
			if target, ok := nv.resolveLocked(m[1]); ok && target != p && !seen[target] {
				seen[target] = true
				n.Links = append(n.Links, target)
				backlinks[target] = append(backlinks[target], p)
			}
		}
	}
	for _, list := range backlinks {
		sort.Strings(list)
	}

	v.mu.Lock()
	v.notes, v.byName, v.byPath, v.tags, v.files, v.dirs = notes, byName, byPath, tags, files, dirs
	v.backlinks = backlinks
	v.indexedAt = time.Now()
	v.mu.Unlock()
	return nil
}

// resolveLocked maps an Obsidian link target ("Note", "folder/Note", "file.canvas")
// to a vault path. Caller must hold the lock (or own an unshared Vault).
func (v *Vault) resolveLocked(target string) (string, bool) {
	t := strings.TrimSpace(target)
	t = strings.TrimSuffix(t, ".md")
	if t == "" {
		return "", false
	}
	if p, ok := v.byPath[fold(t)]; ok {
		return p, true
	}
	if list := v.byName[fold(path.Base(t))]; len(list) > 0 {
		if strings.Contains(t, "/") {
			for _, p := range list {
				if strings.HasSuffix(fold(strings.TrimSuffix(p, ".md")), fold(t)) {
					return p, true
				}
			}
		}
		return list[0], true
	}
	if v.files[target] {
		return target, true
	}
	return "", false
}

func (v *Vault) Resolve(target string) (string, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.resolveLocked(target)
}

func (v *Vault) Note(p string) (*Note, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	n, ok := v.notes[p]
	return n, ok
}

func (v *Vault) Backlinks(p string) []string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return append([]string(nil), v.backlinks[p]...)
}

func (v *Vault) Count() int {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return len(v.notes)
}

// Listing returns the direct subfolders and files of a folder.
func (v *Vault) Listing(dir string) (folders []string, files []string, ok bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	if !v.dirs[dir] {
		return nil, nil, false
	}
	for d := range v.dirs {
		if d != "" && path.Dir(d) == dirOrDot(dir) {
			folders = append(folders, d)
		}
	}
	for f := range v.files {
		if path.Dir(f) == dirOrDot(dir) {
			files = append(files, f)
		}
	}
	sort.Slice(folders, func(i, j int) bool { return fold(folders[i]) < fold(folders[j]) })
	sort.Slice(files, func(i, j int) bool { return fold(files[i]) < fold(files[j]) })
	return folders, files, true
}

func dirOrDot(d string) string {
	if d == "" {
		return "."
	}
	return d
}

// CountIn returns how many notes live under a folder (recursively).
func (v *Vault) CountIn(dir string) int {
	v.mu.RLock()
	defer v.mu.RUnlock()
	c := 0
	for p := range v.notes {
		if strings.HasPrefix(p, dir+"/") {
			c++
		}
	}
	return c
}

func (v *Vault) Recent(limit int) []*Note {
	v.mu.RLock()
	defer v.mu.RUnlock()
	list := make([]*Note, 0, len(v.notes))
	for _, n := range v.notes {
		list = append(list, n)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ModTime.After(list[j].ModTime) })
	if len(list) > limit {
		list = list[:limit]
	}
	return list
}

type TagCount struct {
	Tag   string
	Count int
}

func (v *Vault) Tags() []TagCount {
	v.mu.RLock()
	defer v.mu.RUnlock()
	out := make([]TagCount, 0, len(v.tags))
	for t, ps := range v.tags {
		out = append(out, TagCount{t, len(ps)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Tag < out[j].Tag })
	return out
}

func (v *Vault) Tagged(tag string) []*Note {
	v.mu.RLock()
	defer v.mu.RUnlock()
	var out []*Note
	for t, ps := range v.tags {
		if t == tag || strings.HasPrefix(t, tag+"/") {
			for _, p := range ps {
				out = append(out, v.notes[p])
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return fold(out[i].Title) < fold(out[j].Title) })
	return dedupe(out)
}

func dedupe(in []*Note) []*Note {
	seen := map[string]bool{}
	out := in[:0]
	for _, n := range in {
		if !seen[n.Path] {
			seen[n.Path] = true
			out = append(out, n)
		}
	}
	return out
}

type Hit struct {
	Note    *Note
	Snippet string
	score   int
}

// Search finds notes whose title or body contain every word of the query.
func (v *Vault) Search(q string, limit int) []Hit {
	words := strings.Fields(fold(q))
	if len(words) == 0 {
		return nil
	}
	v.mu.RLock()
	defer v.mu.RUnlock()
	var hits []Hit
	for _, n := range v.notes {
		score := 0
		ok := true
		ft := fold(n.Title)
		for _, w := range words {
			if !strings.Contains(n.search, w) {
				ok = false
				break
			}
			if strings.Contains(ft, w) {
				score += 10
			}
			score += strings.Count(n.search, w)
		}
		if ok {
			hits = append(hits, Hit{Note: n, Snippet: snippet(n.Body, words[0]), score: score})
		}
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].score > hits[j].score })
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits
}

// snippet returns ~160 characters of plain text around the first match.
func snippet(body, word string) string {
	lines := strings.Split(body, "\n")
	for _, l := range lines {
		if strings.Contains(fold(l), word) {
			l = strings.TrimSpace(wikiLinkRe.ReplaceAllStringFunc(l, func(m string) string {
				s := wikiLinkRe.FindStringSubmatch(m)
				if s[3] != "" {
					return s[3]
				}
				return s[1]
			}))
			l = strings.TrimLeft(l, "#>-*| ")
			r := []rune(l)
			if len(r) > 160 {
				r = append(r[:157], '…')
			}
			return string(r)
		}
	}
	return ""
}

// splitFrontmatter removes a leading YAML block and extracts tags and a date.
func splitFrontmatter(src string) (body string, tags []string, date string) {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	if !strings.HasPrefix(src, "---\n") {
		return src, nil, ""
	}
	end := strings.Index(src[4:], "\n---")
	if end < 0 {
		return src, nil, ""
	}
	fm := src[4 : 4+end]
	rest := src[4+end+4:]
	rest = strings.TrimPrefix(rest, "\n")
	inTags := false
	for _, line := range strings.Split(fm, "\n") {
		trim := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "tags:"):
			inTags = true
			if v := strings.TrimSpace(strings.TrimPrefix(line, "tags:")); v != "" {
				v = strings.Trim(v, "[]")
				for _, t := range strings.Split(v, ",") {
					if t = cleanTag(t); t != "" {
						tags = append(tags, t)
					}
				}
				inTags = false
			}
		case inTags && strings.HasPrefix(trim, "- "):
			if t := cleanTag(strings.TrimPrefix(trim, "- ")); t != "" {
				tags = append(tags, t)
			}
		default:
			if trim != "" && !strings.HasPrefix(line, " ") {
				inTags = false
			}
			for _, key := range []string{"actualizado:", "fecha:", "date:"} {
				if date == "" && strings.HasPrefix(line, key) {
					date = strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, key)), `"'`)
				}
			}
		}
	}
	return rest, tags, date
}

func cleanTag(t string) string {
	return strings.TrimPrefix(strings.Trim(strings.TrimSpace(t), `"'`), "#")
}
