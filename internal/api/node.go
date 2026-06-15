package api

import (
	"context"
	"net/http"
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
	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "agent deployment will be implemented with gRPC module",
	})
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
