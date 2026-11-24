package launcher

import (
	"sort"
	"strings"
	"time"
)

// Usage 记录一个条目被启动的次数与最后一次启动时间。
type Usage struct {
	Count int       `json:"count"`
	Last  time.Time `json:"last"`
}

// filter 返回命中查询的条目。查询按空白切成多个词，每个词都必须出现在
// 标题或关键词里（关键词已包含别名与拼音），匹配不区分大小写。
// 空查询返回全部条目。
func filter(entries []Entry, query string) []Entry {
	tokens := strings.Fields(strings.ToLower(query))
	if len(tokens) == 0 {
		return entries
	}
	out := make([]Entry, 0, len(entries))
	for _, e := range entries {
		haystack := strings.ToLower(e.Title + " " + e.Keywords)
		if containsAll(haystack, tokens) {
			out = append(out, e)
		}
	}
	return out
}

// categoryOf 返回条目分类；旧缓存没有该字段时按应用处理。
func categoryOf(e Entry) string {
	if e.Category == "" {
		return categoryApp
	}
	return e.Category
}

// filterCategory 只保留指定分类；category 为空表示全部。
func filterCategory(entries []Entry, category string) []Entry {
	if category == "" {
		return entries
	}
	out := make([]Entry, 0, len(entries))
	for _, e := range entries {
		if categoryOf(e) == category {
			out = append(out, e)
		}
	}
	return out
}

// containsAll 报告 haystack 是否包含全部 token。
func containsAll(haystack string, tokens []string) bool {
	for _, token := range tokens {
		if !strings.Contains(haystack, token) {
			return false
		}
	}
	return true
}

// rank 按使用次数降序、最近使用时间降序、Title 长度升序排序，并截断到
// limit 条。没有任何使用记录时按 Title 字典序取前 limit 条。
func rank(entries []Entry, usage map[string]Usage, limit int) []Entry {
	if len(entries) == 0 || limit <= 0 {
		return nil
	}
	out := append([]Entry(nil), entries...)
	used := false
	for _, e := range out {
		if u, ok := usage[e.Path]; ok && (u.Count > 0 || !u.Last.IsZero()) {
			used = true
			break
		}
	}
	if used {
		sort.SliceStable(out, func(i, j int) bool {
			a, b := usage[out[i].Path], usage[out[j].Path]
			if a.Count != b.Count {
				return a.Count > b.Count
			}
			if !a.Last.Equal(b.Last) {
				return a.Last.After(b.Last)
			}
			if len(out[i].Title) != len(out[j].Title) {
				return len(out[i].Title) < len(out[j].Title)
			}
			return strings.ToLower(out[i].Title) < strings.ToLower(out[j].Title)
		})
	} else {
		sort.SliceStable(out, func(i, j int) bool {
			a, b := strings.ToLower(out[i].Title), strings.ToLower(out[j].Title)
			if a != b {
				return a < b
			}
			return out[i].Path < out[j].Path
		})
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

// loadUsage 读取使用记录；缺失或损坏时保持空表。
func (m *Module) loadUsage() {
	if m.store == nil {
		return
	}
	m.usage = m.store.Usage()
}

// recordUsage 累加一次启动并写回使用记录。
func (m *Module) recordUsage(path string) {
	if m.usage == nil {
		m.usage = make(map[string]Usage)
	}
	u := m.usage[path]
	u.Count++
	u.Last = time.Now()
	m.usage[path] = u
	m.saveUsage()
}

// saveUsage 把使用记录写回数据库；写失败只记 debug 日志。
func (m *Module) saveUsage() {
	if m.store == nil {
		return
	}
	if err := m.store.SaveUsage(m.usage); err != nil && m.log != nil {
		m.log.Debug("launcher: 写入使用记录", "error", err)
	}
}
