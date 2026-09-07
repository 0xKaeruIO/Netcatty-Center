package httpapi

import (
	"encoding/json"
	"net/http"

	"netcatty-center/internal/store"

	"github.com/gin-gonic/gin"
)

type hostExportFile struct {
	Groups []string          `json:"groups"`
	Hosts  []store.HostInput `json:"hosts"`
}

func (s *Server) exportHosts(c *gin.Context) {
	hosts, err := s.store.ListHosts()
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	groups, err := s.store.ListGroups()
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	raw, err := marshalHostExportJSON(groups, hosts)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.Header("Content-Disposition", `attachment; filename="hosts-export.json"`)
	c.Data(http.StatusOK, "application/json; charset=utf-8", raw)
}

func marshalHostExportJSON(groups []string, hosts []store.Host) ([]byte, error) {
	payload := hostExportFile{
		Groups: groups,
		Hosts:  make([]store.HostInput, 0, len(hosts)),
	}
	if payload.Groups == nil {
		payload.Groups = []string{}
	}
	for _, host := range hosts {
		payload.Hosts = append(payload.Hosts, toExportHost(host))
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}

func toExportHost(host store.Host) store.HostInput {
	tags := host.Tags
	if tags == nil {
		tags = []string{}
	}
	rules := host.StartupCommandRules
	if rules == nil {
		rules = []store.StartupCommandRule{}
	}
	keyIDs := host.VisibleKeyIDs
	if keyIDs == nil {
		keyIDs = []string{}
	}
	command := host.StartupCommand
	if host.StartupCommandRunMode == "rules" {
		command = ""
	}
	return store.HostInput{
		Label:                 host.Label,
		Hostname:              host.Hostname,
		Port:                  host.Port,
		Username:              host.Username,
		Group:                 host.Group,
		Tags:                  tags,
		OS:                    host.OS,
		Protocol:              host.Protocol,
		DeviceType:            host.DeviceType,
		Notes:                 host.Notes,
		Password:              host.Password,
		PrivateKey:            host.PrivateKey,
		Passphrase:            host.Passphrase,
		StartupCommand:        command,
		StartupCommandRunMode: host.StartupCommandRunMode,
		StartupCommandRules:   rules,
		Visibility:            host.Visibility,
		VisibleKeyIDs:         keyIDs,
	}
}
