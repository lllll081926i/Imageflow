package upscale

import (
	"fmt"
	"strconv"
	"strings"
)

func validateGPUIds(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || value == "-1" || strings.EqualFold(value, "auto") || value == "自动" {
		return "", nil
	}
	parts := strings.Split(value, ",")
	if len(parts) > 16 {
		return "", newError(CodeBadRequest, "最多指定 16 个 GPU")
	}
	seen := map[int]bool{}
	for index, part := range parts {
		device, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || device < 0 || device > 31 || seen[device] {
			return "", newError(CodeBadRequest, "GPU ID 必须为不重复的 0–31，可用逗号分隔")
		}
		seen[device] = true
		parts[index] = strconv.Itoa(device)
	}
	return strings.Join(parts, ","), nil
}

type inferenceLog struct {
	pending string
	tail    string
	notify  func(int, string)
}

func (log *inferenceLog) Write(data []byte) (int, error) {
	log.tail += string(data)
	if len(log.tail) > 16<<10 {
		log.tail = log.tail[len(log.tail)-(16<<10):]
	}
	log.pending += strings.ReplaceAll(string(data), "\r", "\n")
	for {
		end := strings.IndexByte(log.pending, '\n')
		if end < 0 {
			break
		}
		line := strings.TrimSpace(log.pending[:end])
		log.pending = log.pending[end+1:]
		if line == "" || log.notify == nil {
			continue
		}
		percent := -1
		if strings.HasSuffix(line, "%") {
			var value float64
			if _, err := fmt.Sscanf(line, "%f%%", &value); err == nil && value >= 0 && value <= 100 {
				percent = int(value)
			}
		}
		log.notify(percent, line)
	}
	if len(log.pending) > 4096 {
		log.pending = log.pending[len(log.pending)-4096:]
	}
	return len(data), nil
}

func (log *inferenceLog) String() string { return log.tail }
