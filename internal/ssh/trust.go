package ssh

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/michael/device_grid/internal/model"
)

type KeyPair struct {
	PublicKey  string
	PrivateKey string
}

func GenerateKeyPair() (*KeyPair, error) {
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate ed25519 key: %w", err)
	}

	privPEM, err := ssh.MarshalPrivateKey(privKey, "")
	if err != nil {
		return nil, fmt.Errorf("marshal private key: %w", err)
	}
	privateKey := string(pem.EncodeToMemory(privPEM))

	pub, err := ssh.NewPublicKey(pubKey)
	if err != nil {
		return nil, fmt.Errorf("new public key: %w", err)
	}
	publicKey := string(ssh.MarshalAuthorizedKey(pub))

	return &KeyPair{
		PublicKey:  strings.TrimSpace(publicKey),
		PrivateKey: privateKey,
	}, nil
}

func (m *Manager) EstablishTrust(ctx context.Context, nodeID string) error {
	node, err := m.repos.Nodes().GetByID(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("get node: %w", err)
	}

	if node.PasswordEnc == "" && node.PrivateKeyEnc == "" {
		return fmt.Errorf("节点 %s 没有存储密码或私钥，无法建立授信。请编辑节点填写密码或导入私钥", node.Name)
	}

	kp, err := GenerateKeyPair()
	if err != nil {
		return fmt.Errorf("generate key pair: %w", err)
	}

	// Try all available auth methods: private key first, then password
	client, connectErr := m.dialWithAllMethods(node)
	if connectErr != nil {
		return fmt.Errorf("无法连接到节点（密码和私钥均失败）: %w", connectErr)
	}
	defer client.Close()

	if err := m.installPublicKey(client, kp.PublicKey); err != nil {
		return fmt.Errorf("install public key: %w", err)
	}

	// Store new keypair, keep original credentials for fallback
	privKeyEnc, err := m.enc.EncryptString(kp.PrivateKey)
	if err != nil {
		return fmt.Errorf("encrypt private key: %w", err)
	}

	// Store the host key from the initial connection (TOFU)
	// During trust establishment we accept the key, then store it

	node.AuthMode = "key"
	node.PrivateKeyEnc = privKeyEnc
	node.Status = model.NodeStatusOnline
	node.LastSeenAt = time.Now()
	if err := m.repos.Nodes().Update(ctx, node); err != nil {
		return fmt.Errorf("update node: %w", err)
	}

	// Verify key login works — use TOFU callback for verification
	keySigner, err := ssh.ParsePrivateKey([]byte(kp.PrivateKey))
	if err != nil {
		return fmt.Errorf("parse key for verification: %w", err)
	}
	verifyConfig := &ssh.ClientConfig{
		User:            node.Username,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(keySigner)},
		HostKeyCallback: m.getHostKeyCallback(node),
		Timeout:         m.config.ConnectTimeout,
	}
	addr := fmt.Sprintf("%s:%d", node.Host, node.Port)
	verifyClient, err := ssh.Dial("tcp", addr, verifyConfig)
	if err != nil {
		return fmt.Errorf("公钥已安装但密钥验证登录失败（可能服务器需要重启 sshd 或启用 PubkeyAuthentication）: %w", err)
	}
	verifyClient.Close()

	return nil
}

// dialWithAllMethods tries private key first, then password, then combined
func (m *Manager) dialWithAllMethods(node *model.Node) (*ssh.Client, error) {
	addr := fmt.Sprintf("%s:%d", node.Host, node.Port)
	var lastErr error

	// Strategy 1: Try private key (for servers with PasswordAuthentication no)
	if node.PrivateKeyEnc != "" {
		keyBytes, err := m.enc.DecryptString(node.PrivateKeyEnc)
		if err == nil {
			signer, err := ssh.ParsePrivateKey([]byte(keyBytes))
			if err == nil {
				config := &ssh.ClientConfig{
					User:            node.Username,
					Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
					HostKeyCallback: m.getHostKeyCallback(node),
					Timeout:         m.config.ConnectTimeout,
				}
				if client, err := ssh.Dial("tcp", addr, config); err == nil {
					return client, nil
				} else {
					lastErr = err
				}
			}
		}
	}

	// Strategy 2: Try password + keyboard-interactive
	if node.PasswordEnc != "" {
		password, err := m.enc.DecryptString(node.PasswordEnc)
		if err == nil {
			client, err := dialWithPassword(node.Host, node.Port, node.Username, password, m.config.ConnectTimeout)
			if err == nil {
				return client, nil
			}
			lastErr = err
		}
	}

	// Strategy 3: Try all methods combined
	methods, err := m.getAuthMethods(node)
	if err == nil {
		config := &ssh.ClientConfig{
			User:            node.Username,
			Auth:            methods,
			HostKeyCallback: m.getHostKeyCallback(node),
			Timeout:         m.config.ConnectTimeout,
		}
		client, err := ssh.Dial("tcp", addr, config)
		if err == nil {
			return client, nil
		}
		lastErr = err
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no valid credentials")
	}
	return nil, lastErr
}

func (m *Manager) installPublicKey(client *ssh.Client, publicKey string) error {
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("new session: %w", err)
	}
	defer session.Close()

	cmd := fmt.Sprintf(`
		mkdir -p ~/.ssh && chmod 700 ~/.ssh
		touch ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys
		grep -qF '%s' ~/.ssh/authorized_keys || echo '%s' >> ~/.ssh/authorized_keys
	`, publicKey, publicKey)

	return session.Run(cmd)
}
