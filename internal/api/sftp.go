package api

import (
	"io"
	"path"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/michael/device_grid/internal/ssh"
	"github.com/michael/device_grid/internal/store/repo"
)

type SFTPHandler struct {
	repos  repo.Repositories
	sshMgr *ssh.Manager
}

func NewSFTPHandler(repos repo.Repositories, sshMgr *ssh.Manager) *SFTPHandler {
	return &SFTPHandler{repos: repos, sshMgr: sshMgr}
}

func (h *SFTPHandler) List(c *gin.Context) {
	nodeID := c.Param("id")
	dirPath := c.DefaultQuery("path", "/")
	if dirPath == "" {
		dirPath = "/"
	}
	entries, err := h.sshMgr.SFTPListDir(c.Request.Context(), nodeID, dirPath)
	if err != nil {
		Error(c, 502, "list dir: "+err.Error())
		return
	}
	OK(c, gin.H{"path": dirPath, "entries": entries})
}

func (h *SFTPHandler) Upload(c *gin.Context) {
	nodeID := c.Param("id")
	dirPath := c.DefaultPostForm("path", "/")

	file, err := c.FormFile("file")
	if err != nil {
		BadRequest(c, "no file: "+err.Error())
		return
	}

	src, err := file.Open()
	if err != nil {
		InternalError(c, "open upload: "+err.Error())
		return
	}
	defer src.Close()

	remotePath := path.Join(dirPath, file.Filename)
	if err := h.sshMgr.SFTPUpload(c.Request.Context(), nodeID, remotePath, src); err != nil {
		Error(c, 502, "upload: "+err.Error())
		return
	}
	OK(c, gin.H{"uploaded": file.Filename, "path": remotePath})
}

func (h *SFTPHandler) Download(c *gin.Context) {
	nodeID := c.Param("id")
	filePath := c.Query("path")
	if filePath == "" {
		BadRequest(c, "path is required")
		return
	}

	reader, err := h.sshMgr.SFTPDownload(c.Request.Context(), nodeID, filePath)
	if err != nil {
		Error(c, 502, "download: "+err.Error())
		return
	}
	defer reader.Close()

	fileName := filePath
	if idx := strings.LastIndex(filePath, "/"); idx >= 0 {
		fileName = filePath[idx+1:]
	}

	c.Header("Content-Disposition", "attachment; filename=\""+fileName+"\"")
	c.Status(200)
	io.Copy(c.Writer, reader)
}

func (h *SFTPHandler) Delete(c *gin.Context) {
	nodeID := c.Param("id")
	filePath := c.Query("path")
	if filePath == "" {
		BadRequest(c, "path is required")
		return
	}
	if err := h.sshMgr.SFTPDelete(c.Request.Context(), nodeID, filePath); err != nil {
		Error(c, 502, "delete: "+err.Error())
		return
	}
	OK(c, gin.H{"deleted": true})
}

func (h *SFTPHandler) Mkdir(c *gin.Context) {
	nodeID := c.Param("id")
	var req struct {
		Path string `json:"path" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "path is required")
		return
	}
	if err := h.sshMgr.SFTPMkdir(c.Request.Context(), nodeID, req.Path); err != nil {
		Error(c, 502, "mkdir: "+err.Error())
		return
	}
	OK(c, gin.H{"created": req.Path})
}

func (h *SFTPHandler) Rename(c *gin.Context) {
	nodeID := c.Param("id")
	var req struct {
		OldPath string `json:"old_path" binding:"required"`
		NewPath string `json:"new_path" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request")
		return
	}
	if err := h.sshMgr.SFTPRename(c.Request.Context(), nodeID, req.OldPath, req.NewPath); err != nil {
		Error(c, 502, "rename: "+err.Error())
		return
	}
	OK(c, gin.H{"renamed": true})
}
