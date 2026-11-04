package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/chaitin/panda-wiki/domain"
)

type assistantSheet struct {
	Name string  `json:"name"`
	Rows [][]any `json:"rows"`
}

func spreadsheetSummary(v *domain.AssistantUpload) (string, error) {
	var sheets []assistantSheet
	if e := json.Unmarshal([]byte(v.Text), &sheets); e != nil {
		return "", fmt.Errorf("Excel解析结果无效")
	}
	result := []map[string]any{}
	for _, s := range sheets {
		sample := s.Rows[:min(4, len(s.Rows))]
		result = append(result, map[string]any{"name": s.Name, "data_rows": max(0, len(s.Rows)-1), "sample_including_header": sample})
	}
	b, _ := json.Marshal(map[string]any{"upload_id": v.ID, "name": v.Name, "sheets": result})
	return "\n[上传表格元数据与样例；完整数据由程序读取，不由模型逐格重写]\n" + string(b), nil
}
func (u *AssistantUsecase) resolveWorkbook(ctx context.Context, r *domain.AssistantRun, spec domain.JSONB) error {
	raw, _ := json.Marshal(spec["sheets"])
	var sheets []domain.JSONB
	if e := json.Unmarshal(raw, &sheets); e != nil {
		return fmt.Errorf("Excel工作表规范无效")
	}
	allowed := map[string]bool{}
	for _, s := range sourceList(r.Sources) {
		if s.Kind == "upload" {
			allowed[s.ID] = true
		}
	}
	for _, s := range sheets {
		src, ok := s["source"].(map[string]any)
		if !ok {
			continue
		}
		id, _ := src["upload_id"].(string)
		if !allowed[id] {
			return fmt.Errorf("Excel数据源不属于本轮授权资料")
		}
		v, e := u.GetUpload(ctx, r.KBID, r.UserID, id)
		if e != nil {
			return e
		}
		if v.Format != "xlsx" && v.Format != "csv" {
			return fmt.Errorf("Excel数据源必须为XLSX或CSV")
		}
		var originals []assistantSheet
		if e = json.Unmarshal([]byte(v.Text), &originals); e != nil {
			return e
		}
		sheetName, _ := src["sheet"].(string)
		var original *assistantSheet
		for i := range originals {
			if originals[i].Name == sheetName {
				original = &originals[i]
				break
			}
		}
		if original == nil || len(original.Rows) == 0 {
			return fmt.Errorf("Excel指定的数据工作表不存在或为空")
		}
		names := make([]string, len(original.Rows[0]))
		indexes := map[string]int{}
		for i, c := range original.Rows[0] {
			names[i] = fmt.Sprint(c)
			if _, found := indexes[names[i]]; found {
				return fmt.Errorf("Excel源表含重复列名，请先整理表头")
			}
			indexes[names[i]] = i
		}
		operation, _ := src["operation"].(string)
		if operation == "" {
			operation = "copy"
		}
		switch operation {
		case "copy":
			chosen := names
			if a, ok := src["columns"].([]any); ok && len(a) > 0 {
				chosen = nil
				for _, c := range a {
					chosen = append(chosen, fmt.Sprint(c))
				}
			}
			rows := [][]any{}
			for _, old := range original.Rows[1:] {
				row := []any{}
				for _, name := range chosen {
					index, ok := indexes[name]
					if !ok {
						return fmt.Errorf("Excel字段不存在：%s", name)
					}
					var value any
					if index < len(old) {
						value = old[index]
					}
					row = append(row, value)
				}
				rows = append(rows, row)
			}
			s["columns"] = chosen
			s["rows"] = rows
		case "sum", "count":
			groupBy, _ := src["group_by"].(string)
			valueName, _ := src["value_column"].(string)
			gi, ok := indexes[groupBy]
			if !ok {
				return fmt.Errorf("Excel分组字段不存在")
			}
			vi, vok := indexes[valueName]
			if operation == "sum" && !vok {
				return fmt.Errorf("Excel求和字段不存在")
			}
			sums := map[string]float64{}
			for _, row := range original.Rows[1:] {
				key := ""
				if gi < len(row) && row[gi] != nil {
					key = fmt.Sprint(row[gi])
				}
				if operation == "count" {
					sums[key]++
					continue
				}
				if vi >= len(row) || row[vi] == nil || row[vi] == "" {
					continue
				}
				n, e := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(row[vi])), 64)
				if e != nil {
					return fmt.Errorf("Excel求和字段包含非数字值，请先清洗数据")
				}
				sums[key] += n
			}
			keys := []string{}
			for k := range sums {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			rows := [][]any{}
			for _, k := range keys {
				rows = append(rows, []any{k, sums[k]})
			}
			label := "数量"
			if operation == "sum" {
				label = valueName + "合计"
			}
			s["columns"] = []string{groupBy, label}
			s["rows"] = rows
		default:
			return fmt.Errorf("Excel源表首版支持复制选列、分组求和和计数，请缩小操作范围")
		}
	}
	spec["sheets"] = sheets
	return nil
}
