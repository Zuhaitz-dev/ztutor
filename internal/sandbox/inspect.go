package sandbox

import (
	"fmt"
	"regexp"
	"strings"
)

type FieldInfo struct {
	Name   string
	Offset int
	Size   int
}

type StructInfo struct {
	Name      string
	TotalSize int
	Fields    []FieldInfo
}

var structRegex = regexp.MustCompile(`struct\s+(\w+)\s*\{([^}]+)\}`)

func InspectStruct(code, structName string, lang Language, exec Executor) (*StructInfo, error) {
	matches := structRegex.FindStringSubmatch(code)
	var tag string
	for i := 1; i < len(matches); i += 2 {
		if matches[i] == structName {
			tag = matches[i+1]
			break
		}
	}
	if tag == "" {
		return nil, fmt.Errorf("struct %s not found in source", structName)
	}

	fields := parseFields(tag)
	if len(fields) == 0 {
		return nil, fmt.Errorf("no fields found in struct %s", structName)
	}

	helper := buildInspectHelper(code, structName, fields)
	files := map[string]string{lang.SourceFileName(): helper}
	result, err := exec.Run(lang, files, "", "", nil, nil)
	if err != nil {
		return nil, fmt.Errorf("compilation failed: %w", err)
	}
	if result.Error != "" {
		return nil, fmt.Errorf("compilation error: %s", result.Error)
	}

	return parseInspectOutput(strings.TrimSpace(result.Output), structName, fields)
}

func parseFields(tag string) []string {
	var fields []string
	for _, line := range strings.Split(tag, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		line = strings.TrimRight(line, ";")
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			fields = append(fields, parts[len(parts)-1])
		}
	}
	return fields
}

func buildInspectHelper(code, structName string, fields []string) string {
	var b strings.Builder
	b.WriteString(code)
	b.WriteString("\n#include <stdio.h>\n#include <stddef.h>\n")
	b.WriteString("int main(void) {\n")
	b.WriteString(fmt.Sprintf("  printf(\"sizeof=%%zu\\n\", sizeof(struct %s));\n", structName))
	for _, f := range fields {
		b.WriteString(fmt.Sprintf("  printf(\"%s=%%zu\\n\", offsetof(struct %s, %s));\n", f, structName, f))
	}
	b.WriteString("  return 0;\n}\n")
	return b.String()
}

func parseInspectOutput(output, structName string, fields []string) (*StructInfo, error) {
	info := &StructInfo{Name: structName}
	fieldSizes := make(map[string]int)

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		name := parts[0]
		var val int
		if _, err := fmt.Sscanf(parts[1], "%d", &val); err != nil {
			continue
		}
		if name == "sizeof" {
			info.TotalSize = val
		} else {
			fieldSizes[name] = val
		}
	}

	lastOffset := 0
	lastName := ""
	for _, f := range fields {
		offset, ok := fieldSizes[f]
		if !ok {
			continue
		}
		if lastName != "" {
			fi := info.Fields[len(info.Fields)-1]
			fi.Size = offset - lastOffset
			info.Fields[len(info.Fields)-1] = fi
		}
		info.Fields = append(info.Fields, FieldInfo{Name: f, Offset: offset})
		lastOffset = offset
		lastName = f
	}
	if lastName != "" && info.TotalSize > 0 {
		fi := info.Fields[len(info.Fields)-1]
		fi.Size = info.TotalSize - lastOffset
		info.Fields[len(info.Fields)-1] = fi
	}

	return info, nil
}
