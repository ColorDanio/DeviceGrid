package api

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/michael/device_grid/internal/crypto"
	"github.com/michael/device_grid/internal/model"
	"github.com/michael/device_grid/internal/store/repo"
)

type ImportHandler struct {
	repos repo.Repositories
	enc   *crypto.Encryptor
}

func NewImportHandler(repos repo.Repositories, enc *crypto.Encryptor) *ImportHandler {
	return &ImportHandler{repos: repos, enc: enc}
}

func (h *ImportHandler) DownloadTemplate(c *gin.Context) {
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="devicegrid_nodes_template.csv"`)
	c.String(http.StatusOK, "name,host,port,username,password,private_key,tags\nweb-01,192.168.1.10,22,root,yourpass,,web;production\ndb-01,192.168.1.20,22,root,,/path/to/id_rsa,db;production\n")
}

type importResult struct {
	Total    int      `json:"total"`
	Success  int      `json:"success"`
	Failed   int      `json:"failed"`
	Errors   []string `json:"errors,omitempty"`
	Imported []string `json:"imported_names,omitempty"`
}

func (h *ImportHandler) Import(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		BadRequest(c, "no file uploaded")
		return
	}

	src, err := file.Open()
	if err != nil {
		InternalError(c, "open file: "+err.Error())
		return
	}
	defer src.Close()

	reader := csv.NewReader(src)
	reader.TrimLeadingSpace = true

	records, err := reader.ReadAll()
	if err != nil {
		BadRequest(c, "parse CSV: "+err.Error())
		return
	}

	if len(records) < 2 {
		BadRequest(c, "CSV file is empty or has no data rows")
		return
	}

	header := records[0]
	colMap := make(map[string]int)
	for i, h := range header {
		colMap[strings.ToLower(strings.TrimSpace(h))] = i
	}

	nameIdx, ok := colMap["name"]
	if !ok {
		nameIdx = 0
	}
	hostIdx, hostOK := colMap["host"]
	if !hostOK {
		BadRequest(c, "CSV must have a 'host' column")
		return
	}
	portIdx := colMap["port"]
	userIdx := colMap["username"]
	passIdx := colMap["password"]
	tagIdx := colMap["tags"]
	keyIdx := colMap["private_key"]

	result := importResult{Total: len(records) - 1}

	for i, row := range records[1:] {
		if len(row) == 0 || (len(row) == 1 && row[0] == "") {
			continue
		}

		get := func(idx int, def string) string {
			if idx >= 0 && idx < len(row) {
				v := strings.TrimSpace(row[idx])
				if v != "" {
					return v
				}
			}
			return def
		}

		name := get(nameIdx, fmt.Sprintf("node-%d", i+1))
		host := get(hostIdx, "")
		if host == "" {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("Row %d: missing host", i+2))
			continue
		}

		portStr := get(portIdx, "22")
		port := 22
		fmt.Sscanf(portStr, "%d", &port)
		username := get(userIdx, "root")
		password := get(passIdx, "")
		keyPath := get(keyIdx, "")
		tagStr := get(tagIdx, "")
		var tags []string
		if tagStr != "" {
			for _, t := range strings.FieldsFunc(tagStr, func(r rune) bool { return r == ';' || r == ',' || r == '|' }) {
				t = strings.TrimSpace(t)
				if t != "" {
					tags = append(tags, t)
				}
			}
		}

		passwordEnc := ""
		if password != "" {
			passwordEnc, err = h.enc.EncryptString(password)
			if err != nil {
				result.Failed++
				result.Errors = append(result.Errors, fmt.Sprintf("Row %d (%s): encrypt error", i+2, name))
				continue
			}
		}

		privateKeyEnc := ""
		privateKeyContent := ""
		if keyPath != "" {
			// SECURITY: Do NOT read local files from user-supplied CSV input
			// Instead, treat the value as the PEM key content directly
			if strings.Contains(keyPath, "PRIVATE KEY") {
				privateKeyContent = keyPath
			} else {
				result.Failed++
				result.Errors = append(result.Errors, fmt.Sprintf("Row %d (%s): private_key must contain PEM key content, not a file path", i+2, name))
				continue
			}
		}
		if privateKeyContent != "" {
			privateKeyEnc, err = h.enc.EncryptString(privateKeyContent)
			if err != nil {
				result.Failed++
				result.Errors = append(result.Errors, fmt.Sprintf("Row %d (%s): encrypt key error", i+2, name))
				continue
			}
		}

		authMode := model.AuthPassword
		if privateKeyEnc != "" && passwordEnc == "" {
			authMode = model.AuthKey
		}

		node := &model.Node{
			ID:            uuid.NewString(),
			Name:          name,
			Host:          host,
			Port:          port,
			Username:      username,
			AuthMode:      authMode,
			PasswordEnc:   passwordEnc,
			PrivateKeyEnc: privateKeyEnc,
			TransportMode: model.TransportSSH,
			AgentPort:     9090,
			Status:        model.NodeStatusUntrusted,
			Tags:          tags,
		}

		if err := h.repos.Nodes().Create(c.Request.Context(), node); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("Row %d (%s): %s", i+2, name, err.Error()))
			continue
		}
		result.Success++
		result.Imported = append(result.Imported, name)
	}

	Created(c, result)
}

func (h *ImportHandler) Export(c *gin.Context) {
	nodes, err := h.repos.Nodes().List(c.Request.Context(), model.NodeFilter{})
	if err != nil {
		InternalError(c, "list nodes: "+err.Error())
		return
	}

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="devicegrid_nodes.csv"`)

	writer := csv.NewWriter(c.Writer)
	writer.Write([]string{"name", "host", "port", "username", "tags"})

	for _, n := range nodes {
		writer.Write([]string{
			n.Name,
			n.Host,
			fmt.Sprintf("%d", n.Port),
			n.Username,
			strings.Join(n.Tags, ";"),
		})
	}
	writer.Flush()
}
