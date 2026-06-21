package api

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/michael/device_grid/internal/crypto"
	"github.com/michael/device_grid/internal/model"
	nodepkg "github.com/michael/device_grid/internal/node"
	"github.com/michael/device_grid/internal/ssh"
	"github.com/michael/device_grid/internal/store/repo"
	"github.com/michael/device_grid/internal/transport"
)

type NodeHandler struct {
	repos           repo.Repositories
	enc             *crypto.Encryptor
	transport       *transport.Manager
	sshMgr          *ssh.Manager
	metricsCache    *nodepkg.MetricsCache
	enableGeoLookup bool
}

func NewNodeHandler(repos repo.Repositories, enc *crypto.Encryptor, tm *transport.Manager, sshMgr *ssh.Manager, mc *nodepkg.MetricsCache, enableGeo bool) *NodeHandler {
	return &NodeHandler{repos: repos, enc: enc, transport: tm, sshMgr: sshMgr, metricsCache: mc, enableGeoLookup: enableGeo}
}

type createNodeRequest struct {
	Name      string   `json:"name" binding:"required"`
	Host      string   `json:"host" binding:"required"`
	Port      int      `json:"port"`
	Username  string   `json:"username"`
	Password  string   `json:"password"`
	PrivateKey string  `json:"private_key"`
	Tags      []string `json:"tags"`
}

func (h *NodeHandler) List(c *gin.Context) {
	filter := model.NodeFilter{
		Search: c.Query("search"),
		Status: model.NodeStatus(c.Query("status")),
		Tag:    c.Query("tag"),
	}
	nodes, err := h.repos.Nodes().List(c.Request.Context(), filter)
	if err != nil {
		InternalError(c, "list nodes: "+err.Error())
		return
	}
	OK(c, nodes)
}

func (h *NodeHandler) Get(c *gin.Context) {
	node, err := h.repos.Nodes().GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		NotFound(c, "node not found")
		return
	}
	OK(c, node)
}

func (h *NodeHandler) Create(c *gin.Context) {
	var req createNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request: "+err.Error())
		return
	}

	if req.Port == 0 {
		req.Port = 22
	}
	if req.Username == "" {
		req.Username = "root"
	}

	passwordEnc := ""
	if req.Password != "" {
		var err error
		passwordEnc, err = h.enc.EncryptString(req.Password)
		if err != nil {
			InternalError(c, "encrypt password: "+err.Error())
			return
		}
	}

	privateKeyEnc := ""
	if req.PrivateKey != "" {
		var err error
		privateKeyEnc, err = h.enc.EncryptString(req.PrivateKey)
		if err != nil {
			InternalError(c, "encrypt private key: "+err.Error())
			return
		}
	}

	authMode := model.AuthPassword
	if privateKeyEnc != "" && passwordEnc == "" {
		authMode = model.AuthKey
	}

	node := &model.Node{
		ID:            uuid.NewString(),
		Name:          req.Name,
		Host:          req.Host,
		Port:          req.Port,
		Username:      req.Username,
		AuthMode:      authMode,
		PasswordEnc:   passwordEnc,
		PrivateKeyEnc: privateKeyEnc,
		TransportMode: model.TransportSSH,
		AgentPort:     9090,
		Status:        model.NodeStatusUntrusted,
		Tags:          req.Tags,
	}

	if h.enableGeoLookup {
		if geo, err := nodepkg.LookupGeo(req.Host); err == nil && geo != nil {
			node.Country = geo.Country
			node.CountryCode = geo.CountryCode
			node.Region = geo.City
			node.ISP = geo.ISP
		}
	}

	if err := h.repos.Nodes().Create(c.Request.Context(), node); err != nil {
		InternalError(c, "create node: "+err.Error())
		return
	}
	Created(c, node)
}

func (h *NodeHandler) Update(c *gin.Context) {
	node, err := h.repos.Nodes().GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		NotFound(c, "node not found")
		return
	}

	var req struct {
		Name      *string  `json:"name"`
		Host      *string  `json:"host"`
		Port      *int     `json:"port"`
		Username  *string  `json:"username"`
		Password  *string  `json:"password"`
		PrivateKey *string `json:"private_key"`
		Tags      []string `json:"tags"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request: "+err.Error())
		return
	}

	if req.Name != nil {
		node.Name = *req.Name
	}
	if req.Host != nil {
		node.Host = *req.Host
	}
	if req.Port != nil {
		node.Port = *req.Port
	}
	if req.Username != nil {
		node.Username = *req.Username
	}
	if req.Password != nil && *req.Password != "" {
		encPass, err := h.enc.EncryptString(*req.Password)
		if err != nil {
			InternalError(c, "encrypt password: "+err.Error())
			return
		}
		node.PasswordEnc = encPass
		node.AuthMode = model.AuthPassword
	}
	if req.PrivateKey != nil && *req.PrivateKey != "" {
		encKey, err := h.enc.EncryptString(*req.PrivateKey)
		if err != nil {
			InternalError(c, "encrypt private key: "+err.Error())
			return
		}
		node.PrivateKeyEnc = encKey
		if node.PasswordEnc == "" {
			node.AuthMode = model.AuthKey
		}
	}
	if req.Tags != nil {
		node.Tags = req.Tags
	}

	if err := h.repos.Nodes().Update(c.Request.Context(), node); err != nil {
		InternalError(c, "update node: "+err.Error())
		return
	}
	OK(c, node)
}

func (h *NodeHandler) Delete(c *gin.Context) {
	if err := h.repos.Nodes().Delete(c.Request.Context(), c.Param("id")); err != nil {
		InternalError(c, "delete node: "+err.Error())
		return
	}
	OK(c, gin.H{"deleted": true})
}

func (h *NodeHandler) Health(c *gin.Context) {
	nodeID := c.Param("id")
	node, err := h.repos.Nodes().GetByID(c.Request.Context(), nodeID)
	if err != nil {
		NotFound(c, "node not found")
		return
	}

	err = h.transport.Ping(c.Request.Context(), nodeID)
	if err != nil {
		_ = h.repos.Nodes().UpdateStatus(c.Request.Context(), nodeID, model.NodeStatusOffline)
		OK(c, gin.H{"node_id": nodeID, "status": "offline", "error": err.Error()})
		return
	}

	_ = h.repos.Nodes().UpdateStatus(c.Request.Context(), nodeID, model.NodeStatusOnline)
	OK(c, gin.H{"node_id": nodeID, "name": node.Name, "status": "online"})
}

func (h *NodeHandler) Trust(c *gin.Context) {
	nodeID := c.Param("id")

	node, err := h.repos.Nodes().GetByID(c.Request.Context(), nodeID)
	if err != nil {
		NotFound(c, "node not found")
		return
	}

	if node.PasswordEnc == "" {
		Error(c, http.StatusBadRequest, "该节点没有存储密码，无法建立授信。请编辑节点并填写密码后再试")
		return
	}

	if err := h.sshMgr.EstablishTrust(c.Request.Context(), nodeID); err != nil {
		errMsg := err.Error()
		reason := "未知错误"
		if strings.Contains(errMsg, "no password stored") {
			reason = "节点未存储密码，请编辑节点填写密码"
		} else if strings.Contains(errMsg, "i/o timeout") || strings.Contains(errMsg, "connection refused") || strings.Contains(errMsg, "no route to host") {
			reason = "无法连接到节点（网络不通或端口未开放）"
		} else if strings.Contains(errMsg, "unable to authenticate") || strings.Contains(errMsg, "handshake failed") {
			reason = "密码认证失败（密码错误或服务器拒绝密码登录）"
		} else if strings.Contains(errMsg, "decrypt password") {
			reason = "密码解密失败（服务可能已重启，请重新导入节点）"
		} else if strings.Contains(errMsg, "verify key login failed") {
			reason = "公钥已安装但密钥登录验证失败（服务器可能禁用了 PubkeyAuthentication）"
		}
		Error(c, http.StatusBadGateway, "授信失败: "+reason+"\n详情: "+errMsg)
		return
	}

	OK(c, gin.H{
		"node_id": nodeID,
		"name":    node.Name,
		"status":  "trusted",
		"message": "SSH 密钥授信通道建立成功",
	})

	// After trust, refresh geo data if missing
	if h.enableGeoLookup && node.CountryCode == "" {
		go func() {
			if geo, err := nodepkg.LookupGeo(node.Host); err == nil && geo != nil {
				updated, _ := h.repos.Nodes().GetByID(context.Background(), nodeID)
				if updated != nil {
					updated.Country = geo.Country
					updated.CountryCode = geo.CountryCode
					updated.Region = geo.City
					updated.ISP = geo.ISP
					h.repos.Nodes().Update(context.Background(), updated)
				}
			}
		}()
	}
}

func (h *NodeHandler) DeployAgent(c *gin.Context) {
	nodeID := c.Param("id")
	node, err := h.repos.Nodes().GetByID(c.Request.Context(), nodeID)
	if err != nil {
		NotFound(c, "node not found")
		return
	}

	if node.AuthMode != "key" && node.PasswordEnc == "" {
		BadRequest(c, "节点需要先完成授信才能部署 Agent")
		return
	}

	// Step 1: Detect architecture
	result, err := h.transport.Exec(c.Request.Context(), nodeID, "uname -m")
	if err != nil {
		Error(c, http.StatusBadGateway, "检测架构失败: "+err.Error())
		return
	}
	arch := strings.TrimSpace(result.Stdout)
	agentArch := "amd64"
	if strings.Contains(arch, "aarch64") || strings.Contains(arch, "arm64") {
		agentArch = "arm64"
	}

	// Step 2: Find local agent binary
	agentPath := fmt.Sprintf("bin/devicegrid-agent-linux-%s", agentArch)
	if _, err := os.Stat(agentPath); err != nil {
		// Fallback: try bin/devicegrid-agent (local arch)
		agentPath = "bin/devicegrid-agent"
		if _, err := os.Stat(agentPath); err != nil {
			// Try dist/
			agentPath = fmt.Sprintf("dist/agent-linux-%s", agentArch)
			if _, err := os.Stat(agentPath); err != nil {
				Error(c, http.StatusInternalServerError, fmt.Sprintf("Agent 二进制不存在 (尝试: bin/devicegrid-agent-linux-%s, bin/devicegrid-agent)。请先运行 make build-agent-all", agentArch))
				return
			}
		}
	}

	// Step 3: Upload agent binary via SFTP
	file, err := os.Open(agentPath)
	if err != nil {
		InternalError(c, "打开 agent 二进制失败: "+err.Error())
		return
	}
	defer file.Close()

	remotePath := "/usr/local/bin/devicegrid-agent"
	uploadErr := h.sshMgr.SFTPUpload(c.Request.Context(), nodeID, remotePath, file)
	if uploadErr != nil {
		// Fallback: use base64 transfer via exec
		uploadErr = h.uploadAgentViaBase64(c.Request.Context(), nodeID, agentPath, remotePath)
		if uploadErr != nil {
			Error(c, http.StatusBadGateway, "上传 Agent 失败: "+uploadErr.Error())
			return
		}
	}

	// Step 4: Create systemd service and start
	serverHost := c.Request.Host
	if idx := strings.LastIndex(serverHost, ":"); idx > 0 {
		serverHost = serverHost[:idx]
	}
	if serverHost == "" || serverHost == "localhost" {
		// Try to get server's outbound IP
		serverHost = h.getServerIP()
	}

	systemdScript := fmt.Sprintf(`
cat > /etc/systemd/system/devicegrid-agent.service << 'EOF'
[Unit]
Description=DeviceGrid Agent
After=network.target
Wants=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/devicegrid-agent -server %s:9090 -node-id %s -node-name '%s'
Restart=always
RestartSec=5
# Resource limits — keep agent lightweight
MemoryMax=64M
CPUQuota=5%%
LimitNOFILE=128
Nice=10

[Install]
WantedBy=multi-user.target
EOF

chmod +x %s
systemctl daemon-reload
systemctl enable devicegrid-agent
systemctl restart devicegrid-agent
sleep 1
systemctl is-active devicegrid-agent
`, serverHost, nodeID, node.Name, remotePath)

	result, err = h.transport.Exec(c.Request.Context(), nodeID, systemdScript)
	if err != nil {
		Error(c, http.StatusBadGateway, "systemd 注册失败: "+err.Error())
		return
	}

	// Update node transport mode
	node.TransportMode = model.TransportAgent
	node.AgentPort = 9090
	_ = h.repos.Nodes().Update(c.Request.Context(), node)

	active := strings.TrimSpace(result.Stdout)
	status := "deployed"
	if active != "active" {
		status = "installed_but_inactive"
	}

	OK(c, gin.H{
		"node_id":   nodeID,
		"name":      node.Name,
		"arch":      agentArch,
		"binary":    agentPath,
		"server":    fmt.Sprintf("%s:9090", serverHost),
		"status":    status,
		"active":    active,
		"output":    result.Stdout,
		"message":   fmt.Sprintf("Agent 已部署到 %s (架构: %s, systemd: %s)", node.Name, arch, active),
	})
}

// uploadAgentViaBase64 is a fallback when SFTP is unavailable
func (h *NodeHandler) uploadAgentViaBase64(ctx context.Context, nodeID, localPath, remotePath string) error {
	data, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("read agent binary: %w", err)
	}

	// Upload in 512KB chunks via base64
	chunkSize := 393216 // 512KB raw → ~682KB base64
	for i := 0; i < len(data); i += chunkSize {
		end := i + chunkSize
		if end > len(data) {
			end = len(data)
		}
		chunk := data[i:end]
		encoded := base64.StdEncoding.EncodeToString(chunk)

		if i == 0 {
			// First chunk — create file
			_, err = h.transport.Exec(ctx, nodeID, fmt.Sprintf("echo '%s' | base64 -d > %s", encoded, remotePath))
		} else {
			// Append
			_, err = h.transport.Exec(ctx, nodeID, fmt.Sprintf("echo '%s' | base64 -d >> %s", encoded, remotePath))
		}
		if err != nil {
			return fmt.Errorf("upload chunk %d: %w", i/chunkSize, err)
		}
	}

	// Make executable
	_, err = h.transport.Exec(ctx, nodeID, fmt.Sprintf("chmod +x %s", remotePath))
	return err
}

func (h *NodeHandler) getServerIP() string {
	// Try to determine outbound IP by checking what the node would see
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "localhost"
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}

func (h *NodeHandler) Facts(c *gin.Context) {
	nodeID := c.Param("id")
	facts, err := h.transport.Facts(c.Request.Context(), nodeID)
	if err != nil {
		Error(c, http.StatusBadGateway, "gather facts failed: "+err.Error())
		return
	}

	node, _ := h.repos.Nodes().GetByID(c.Request.Context(), nodeID)
	if node != nil {
		node.OS = facts.OS
		node.Arch = facts.Arch
		node.DockerVersion = facts.DockerVersion
		_ = h.repos.Nodes().Update(c.Request.Context(), node)
	}

	OK(c, facts)
}

func (h *NodeHandler) Metrics(c *gin.Context) {
	nodeID := c.Param("id")

	// Return cached metrics first (instant)
	if h.metricsCache != nil {
		if cached, ok := h.metricsCache.Get(nodeID); ok {
			OK(c, cached.Data)
			return
		}
	}

	// Fallback: fetch on demand
	metrics, err := h.transport.Metrics(c.Request.Context(), nodeID)
	if err != nil {
		Error(c, http.StatusBadGateway, "gather metrics failed: "+err.Error())
		return
	}
	OK(c, metrics)
}

func (h *NodeHandler) RefreshGeo(c *gin.Context) {
	nodeID := c.Param("id")
	node, err := h.repos.Nodes().GetByID(c.Request.Context(), nodeID)
	if err != nil {
		NotFound(c, "node not found")
		return
	}

	if !h.enableGeoLookup {
		OK(c, gin.H{"message": "geo lookup disabled (internal network mode)"})
		return
	}

	geo, err := nodepkg.LookupGeo(node.Host)
	if err != nil || geo == nil {
		Error(c, http.StatusBadGateway, "geo lookup failed: "+err.Error())
		return
	}

	node.Country = geo.Country
	node.CountryCode = geo.CountryCode
	node.Region = geo.City
	node.ISP = geo.ISP
	if err := h.repos.Nodes().Update(c.Request.Context(), node); err != nil {
		InternalError(c, "update node geo: "+err.Error())
		return
	}

	OK(c, gin.H{
		"country":      geo.Country,
		"country_code": geo.CountryCode,
		"region":       geo.City,
		"isp":          geo.ISP,
	})
}

func (h *NodeHandler) TopProcesses(c *gin.Context) {
	nodeID := c.Param("id")
	result, err := h.transport.Exec(c.Request.Context(), nodeID,
		`ps aux --sort=-%cpu | head -11 | tail -10`)
	if err != nil {
		Error(c, http.StatusBadGateway, "get processes failed: "+err.Error())
		return
	}
	OK(c, gin.H{"output": result.Stdout})
}

func (h *NodeHandler) LoginHistory(c *gin.Context) {
	nodeID := c.Param("id")
	result, err := h.transport.Exec(c.Request.Context(), nodeID,
		`last -10 -w 2>/dev/null | head -10`)
	if err != nil {
		Error(c, http.StatusBadGateway, "get login history failed: "+err.Error())
		return
	}
	OK(c, gin.H{"output": result.Stdout})
}
