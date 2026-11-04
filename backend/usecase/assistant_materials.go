package usecase

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/chaitin/panda-wiki/domain"
)

type assistantMaterialCoverage struct {
	Characters int    `json:"characters"`
	Total      int    `json:"total"`
	Mode       string `json:"mode"`
	Hash       string `json:"hash"`
}

type assistantMaterialInput struct {
	Source       domain.AssistantSource
	Header       string
	Text         string
	Hints        []string
	Structured   bool
	MetadataOnly bool
}

func assistantReleaseMetadataOnly(release *domain.NodeRelease) bool {
	if release.Meta.Extraction != nil && release.Meta.Extraction.Scope == "metadata" {
		return true
	}
	// Legacy files may predate the structured extraction report. A known
	// metadata fallback must not become factual evidence merely due to length.
	return strings.Contains(release.Content, "系统当前已索引该文件的基础元信息") ||
		(release.Meta.FileKey != "" && regexp.MustCompile(`(?i)^(mp4|mov|avi|mkv|zip)$`).MatchString(release.Meta.FileType))
}

// Preserve whole small sources and select explicit, labeled windows from large
// ones. A bounded plan is a reading map, not a claim that every page was read.
func assistantMaterialExcerpt(text string, hints []string, budget int) (string, int, string) {
	runes := []rune(text)
	if len(runes) <= budget {
		return text, len(runes), "full"
	}
	if budget <= 0 {
		return "", 0, "unread"
	}
	type span struct{ start, end int }
	spans := []span{}
	add := func(start, size int) {
		start = max(0, min(start, len(runes)))
		end := min(start+size, len(runes))
		if end > start {
			spans = append(spans, span{start, end})
		}
	}
	head := min(1000, max(1, budget/4))
	add(0, head)
	remaining := budget - head
	positions := []int{}
	for _, hint := range hints {
		hint = strings.TrimSpace(hint)
		if hint == "" {
			continue
		}
		if len([]rune(hint)) > 100 {
			hint = string([]rune(hint)[:100])
		}
		if at := strings.Index(text, hint); at >= 0 {
			positions = append(positions, len([]rune(text[:at])))
		}
		if len(positions) == 6 {
			break
		}
	}
	if len(positions) == 0 {
		positions = []int{len(runes) / 3, len(runes) * 2 / 3, max(0, len(runes)-remaining/3)}
	}
	for i, pos := range positions {
		size := remaining / (len(positions) - i)
		add(max(0, pos-size/4), size)
		remaining -= size
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].start < spans[j].start })
	merged := []span{}
	for _, s := range spans {
		if len(merged) > 0 && s.start <= merged[len(merged)-1].end {
			merged[len(merged)-1].end = max(merged[len(merged)-1].end, s.end)
		} else {
			merged = append(merged, s)
		}
	}
	var out strings.Builder
	provided := 0
	for _, s := range merged {
		fmt.Fprintf(&out, "\n[正文节选 %d–%d / %d]\n", s.start+1, s.end, len(runes))
		out.WriteString(string(runes[s.start:s.end]))
		provided += s.end - s.start
	}
	return out.String(), provided, "excerpt"
}

func prepareAssistantMaterials(inputs []assistantMaterialInput, budget int) (string, map[string]assistantMaterialCoverage) {
	coverage := map[string]assistantMaterialCoverage{}
	reserve := 0
	for _, input := range inputs {
		reserve += len([]rune(input.Header)) + 180
		if input.Structured {
			reserve += len([]rune(input.Text))
		}
	}
	remaining := max(0, budget-reserve)
	count := 0
	for _, input := range inputs {
		if !input.Structured {
			count++
		}
	}
	var out strings.Builder
	for _, input := range inputs {
		text, provided, mode := input.Text, len([]rune(input.Text)), "full"
		if !input.Structured {
			quota := remaining / max(1, count)
			count--
			bodyBudget := max(0, quota-240)
			text, provided, mode = assistantMaterialExcerpt(input.Text, input.Hints, bodyBudget)
			remaining = max(0, remaining-len([]rune(text)))
		}
		if input.MetadataOnly {
			mode = "metadata"
		}
		clean := regexp.MustCompile(`\[正文节选 [^\]]+\]`).ReplaceAllString(text, "")
		coverage[assistantSourceKey(input.Source)] = assistantMaterialCoverage{Characters: min(assistantTextLength(clean), provided), Total: assistantTextLength(input.Text), Mode: mode, Hash: input.Source.Hash}
		out.WriteString(input.Header)
		if mode == "excerpt" || mode == "unread" {
			out.WriteString("\n[本轮仅阅读部分可用正文，不能宣称完整覆盖；正式章节可按来源继续读取。]\n")
		}
		out.WriteString(text)
	}
	return out.String(), coverage
}

// Read the pinned source needed for the current chapter, within the original
// frozen source set. No new search, storage download, or scope expansion.
func (u *AssistantUsecase) assistantPartMaterials(ctx context.Context, r *domain.AssistantRun, part domain.AssistantOutlineItem, budget int, supplied ...string) (string, error) {
	alreadyIncluded := func(body string) bool {
		if strings.TrimSpace(body) == "" {
			return false
		}
		for _, text := range supplied {
			if strings.Contains(text, body) {
				return true
			}
		}
		return false
	}
	wanted := map[string]bool{}
	for _, id := range part.SourceIDs {
		wanted[id] = true
	}
	inputs := []assistantMaterialInput{}
	for _, source := range sourceList(r.Sources) {
		if !wanted[assistantSourceKey(source)] || source.Hash == "" {
			continue
		}
		switch source.Kind {
		case "node":
			if _, e := u.authorizedNode(ctx, r.KBID, r.UserID, source.ID); e != nil {
				return "", e
			}
			release, e := u.chat.nodeRepo.GetNodeReleaseByID(ctx, source.Version)
			if e != nil || release.NodeID != source.ID || release.KBID != r.KBID {
				return "", ErrAssistantAccess
			}
			if digest([]byte(release.Content)) != source.Hash {
				return "", fmt.Errorf("资料内容已更新，请重新生成以使用当前版本")
			}
			if assistantReleaseMetadataOnly(release) {
				continue
			}
			if budget <= 0 {
				continue
			}
			// Deduplicate against the actual current request, never an old
			// coverage flag. Hash and permission checks still run first.
			if alreadyIncluded(release.Content) {
				continue
			}
			inputs = append(inputs, assistantMaterialInput{Source: source, Header: "\n[本部分依据：" + source.Name + "；资料ID：" + assistantSourceKey(source) + "]\n", Text: release.Content, Hints: []string{part.Title, part.Purpose}})
		case "upload":
			v, e := u.GetUpload(ctx, r.KBID, r.UserID, source.ID)
			if e != nil {
				return "", e
			}
			if v.Hash != source.Hash {
				return "", ErrAssistantAccess
			}
			if budget <= 0 {
				continue
			}
			if v.Format == "xlsx" || v.Format == "csv" || v.Format == "png" || v.Format == "jpg" || v.Format == "jpeg" {
				continue
			}
			if alreadyIncluded(v.Text) {
				continue
			}
			inputs = append(inputs, assistantMaterialInput{Source: source, Header: "\n[本部分附件依据：" + source.Name + "；资料ID：" + assistantSourceKey(source) + "]\n", Text: v.Text, Hints: []string{part.Title, part.Purpose}})
		}
	}
	text, _ := prepareAssistantMaterials(inputs, budget)
	return text, nil
}
