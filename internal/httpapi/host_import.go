package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"netcatty-center/internal/store"

	"github.com/gin-gonic/gin"
)

const maxHostImportCount = 5000

func (s *Server) importHosts(c *gin.Context) {
	raw, err := io.ReadAll(io.LimitReader(c.Request.Body, 8<<20))
	if err != nil {
		fail(c, http.StatusBadRequest, "无法读取导入内容")
		return
	}
	groups, inputs, err := parseHostImportJSON(raw)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	hosts, err := s.store.ImportHosts(inputs, groups)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	allGroups, err := s.store.ListGroups()
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"hosts":    hosts,
		"groups":   allGroups,
		"imported": len(hosts),
	})
}

func parseHostImportJSON(raw []byte) ([]string, []store.HostInput, error) {
	trimmed := bytes.TrimPrefix(bytes.TrimSpace(raw), []byte("\xef\xbb\xbf"))
	if len(trimmed) == 0 {
		return nil, nil, errors.New("JSON 文件为空")
	}
	if trimmed[0] == '[' {
		var hosts []json.RawMessage
		if err := json.Unmarshal(trimmed, &hosts); err != nil {
			return nil, nil, errors.New("JSON 无效：hosts 数组无法解析")
		}
		inputs, err := hostInputsFromRawList(hosts)
		return nil, inputs, err
	}
	if trimmed[0] != '{' {
		return nil, nil, errors.New("JSON 无效：需要对象或主机数组")
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &obj); err != nil {
		return nil, nil, errors.New("JSON 文件无效")
	}
	groups := asTags(obj["groups"])
	if _, hasHosts := obj["hosts"]; hasHosts {
		var hosts []json.RawMessage
		if err := json.Unmarshal(obj["hosts"], &hosts); err != nil {
			return nil, nil, errors.New("JSON 无效：hosts 必须是数组")
		}
		inputs, err := hostInputsFromRawList(hosts)
		return groups, inputs, err
	}
	if asString(obj["label"]) != "" || asString(obj["hostname"]) != "" {
		return groups, []store.HostInput{hostInputFromRaw(obj)}, nil
	}
	if len(groups) == 0 {
		return nil, nil, errors.New("JSON 中没有 hosts 或 groups")
	}
	return groups, nil, nil
}

func hostInputsFromRawList(hosts []json.RawMessage) ([]store.HostInput, error) {
	if len(hosts) > maxHostImportCount {
		return nil, fmt.Errorf("一次最多导入 %d 台主机", maxHostImportCount)
	}
	inputs := make([]store.HostInput, 0, len(hosts))
	for i, raw := range hosts {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(raw, &obj); err != nil {
			return nil, fmt.Errorf("第 %d 台主机不是对象", i+1)
		}
		inputs = append(inputs, hostInputFromRaw(obj))
	}
	return inputs, nil
}
