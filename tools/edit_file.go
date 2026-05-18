package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EditFile 对文件做编辑，支持两种模式:
//
// 行号模式（推荐）:
//
//	start_line (int) 起始行号（1-indexed，与 Read 输出一致）
//	end_line   (int) 结束行号（含），默认等于 start_line
//	new_string (string) 替换内容
//
// 字符串模式:
//
//	old_string  (string) 要替换的内容（需精确匹配）
//	new_string  (string) 替换为
//	replace_all (bool)   是否替换所有匹配，默认 false
func EditFile(args map[string]any) ToolResult {
	path, _ := args["path"].(string)
	if path == "" {
		return ToolResult{Output: "错误: path 参数为空", Success: false}
	}
	oldStr, _ := args["old_string"].(string)
	newStr, _ := args["new_string"].(string)
	if newStr == "" && oldStr == "" {
		return ToolResult{Output: "错误: new_string 不能为空", Success: false}
	}
	replaceAll, _ := args["replace_all"].(bool)
	startLine := toInt(args["start_line"], 0)

	absPath, err := filepath.Abs(path)
	if err != nil {
		return ToolResult{Output: fmt.Sprintf("路径错误: %v", err), Success: false}
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		return ToolResult{Output: fmt.Sprintf("读取失败: %v", err), Success: false}
	}

	// 行号模式:直接用 Read 输出的行号定位,比 old_string 匹配更可靠。
	if startLine > 0 {
		return editByLine(data, absPath, startLine, args, newStr)
	}

	// 字符串模式:需要 old_string 精确匹配文件内容。
	if oldStr == "" {
		return ToolResult{Output: "错误: old_string 不能为空,或提供 start_line 使用行号模式", Success: false}
	}
	if oldStr == newStr {
		return ToolResult{Output: "错误: new_string 必须与 old_string 不同", Success: false}
	}
	content := string(data)
	count := strings.Count(content, oldStr)
	if count == 0 {
		return ToolResult{Output: "错误: 在文件中未找到 old_string", Success: false}
	}
	if count > 1 && !replaceAll {
		return ToolResult{
			Output:  fmt.Sprintf("错误: old_string 出现 %d 次，请提供更长上下文或设置 replace_all=true", count),
			Success: false,
		}
	}

	var updated string
	if replaceAll {
		updated = strings.ReplaceAll(content, oldStr, newStr)
	} else {
		updated = strings.Replace(content, oldStr, newStr, 1)
	}
	if err := os.WriteFile(absPath, []byte(updated), 0o644); err != nil {
		return ToolResult{Output: fmt.Sprintf("写入失败: %v", err), Success: false}
	}
	return ToolResult{
		Output:  fmt.Sprintf("已替换 %d 处 -> %s", count, absPath),
		Success: true,
	}
}

// editByLine 按行号替换:替换 lines[startLine-1 : endLine] 为 newStr。
func editByLine(data []byte, absPath string, startLine int, args map[string]any, newStr string) ToolResult {
	lines := strings.Split(string(data), "\n")
	if startLine < 1 || startLine > len(lines) {
		return ToolResult{
			Output:  fmt.Sprintf("错误: start_line=%d 超出范围 [1, %d]", startLine, len(lines)),
			Success: false,
		}
	}
	endLine := toInt(args["end_line"], startLine)
	if endLine < startLine {
		return ToolResult{
			Output:  fmt.Sprintf("错误: end_line=%d < start_line=%d", endLine, startLine),
			Success: false,
		}
	}
	if endLine > len(lines) {
		endLine = len(lines)
	}

	replacementLines := strings.Split(newStr, "\n")
	var out []string
	out = append(out, lines[:startLine-1]...)
	out = append(out, replacementLines...)
	out = append(out, lines[endLine:]...)
	updated := strings.Join(out, "\n")

	if err := os.WriteFile(absPath, []byte(updated), 0o644); err != nil {
		return ToolResult{Output: fmt.Sprintf("写入失败: %v", err), Success: false}
	}
	replaced := endLine - startLine + 1
	return ToolResult{
		Output:  fmt.Sprintf("已替换第 %d-%d 行 (%d 行) -> %s", startLine, endLine, replaced, absPath),
		Success: true,
	}
}
