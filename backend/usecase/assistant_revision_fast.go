package usecase

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/chaitin/panda-wiki/domain"
)

type assistantLiteralEdit struct {
	kind, title string
	bullet      int
}

// Only unambiguous, literal commands bypass the model. Compound instructions,
// negative commands and requests to invent a title stay on the model path.
func assistantLiteralRevision(message string) (assistantLiteralEdit, bool) {
	text := strings.TrimSpace(message)
	text = regexp.MustCompile(`^(?:请|帮我|只|仅|把|将|在)+`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`^第[0-9一二三四五六七八九十两]+(?:页|章|节|部分)(?:的|中|里)?`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`^(?:所选|当前)(?:页面|章节|部分)(?:的|中|里)?`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`[，,](?:其余|其他)(?:页面|部分|内容)?(?:保持)?不变[。.!！]?$`).ReplaceAllString(text, "")
	text = strings.TrimSpace(text)
	if match := regexp.MustCompile(`^(?:删除|删掉|去掉|移除)第([0-9一二三四五六七八九十两]+)(?:个)?(?:要点|条)[。.!！]?$`).FindStringSubmatch(text); len(match) > 1 {
		return assistantLiteralEdit{kind: "remove_bullet", bullet: assistantOrdinal(match[1])}, true
	}
	if match := regexp.MustCompile(`^标题(?:改为|改成|修改为|修改成|设置为)[：: ]*(.+)$`).FindStringSubmatch(text); len(match) > 1 {
		title := strings.TrimSpace(match[1])
		quoted := false
		for _, pair := range [][2]string{{"“", "”"}, {"‘", "’"}, {"「", "」"}, {"\"", "\""}, {"'", "'"}} {
			if strings.HasPrefix(title, pair[0]) && strings.HasSuffix(title, pair[1]) {
				title = strings.TrimSuffix(strings.TrimPrefix(title, pair[0]), pair[1])
				quoted = true
				break
			}
		}
		if !quoted && strings.ContainsAny(title, "，,；;。.!！?？\n") {
			return assistantLiteralEdit{}, false
		}
		if !quoted && regexp.MustCompile(`然后|并且|同时|顺便|再把|再将|并(?:重写|精简|删除|修改|调整|替换|加入|补充)|更简洁|更简短|更醒目|更好|更专业|更有吸引力`).MatchString(title) {
			return assistantLiteralEdit{}, false
		}
		return assistantLiteralEdit{kind: "rename", title: strings.TrimSpace(title)}, true
	}
	return assistantLiteralEdit{}, false
}

func assistantApplyLiteralRevision(req domain.AssistantSubmit, base *domain.AssistantArtifact, targets []domain.JSONB) (domain.JSONB, bool, error) {
	command, ok := assistantLiteralRevision(req.Message)
	if !ok {
		return nil, false, nil
	}
	if len(targets) != 1 {
		return nil, true, fmt.Errorf("请先选择一个页面或章节，我会只修改选中部分")
	}
	part := targets[0]
	patch := domain.JSONB{}
	switch command.kind {
	case "rename":
		if command.title == "" || len([]rune(command.title)) > 50 {
			return nil, true, fmt.Errorf("标题请填写1～50个字，原文件已保留")
		}
		patch["title"] = command.title
	case "remove_bullet":
		var bullets []string
		if decodeAssistantValue(part["bullets"], &bullets) != nil || command.bullet < 1 || command.bullet > len(bullets) {
			return nil, true, fmt.Errorf("所选部分没有对应的第%d个要点，请确认删除范围", command.bullet)
		}
		if len(bullets) == 1 {
			return nil, true, fmt.Errorf("所选部分只有一个要点，请修改内容或删除整个页面；原文件已保留")
		}
		// The rendered table/chart takes precedence over bullets; do not claim
		// success for changing a hidden body beside an existing visual.
		for _, key := range []string{"table", "chart", "image"} {
			if part[key] != nil {
				return nil, true, fmt.Errorf("所选页面主要内容是表格或图片，请明确要修改的内容")
			}
		}
		patch["bullets"] = append(bullets[:command.bullet-1], bullets[command.bullet:]...)
	}
	changed := assistantMergePartFields(part, patch)
	merged, err := mergeAssistantRevision(base, []domain.JSONB{changed})
	if err != nil {
		return nil, true, err
	}
	merged["answer"] = "已按要求修改所选部分，其余内容保持原样，原版本已保留。"
	return merged, true, nil
}
