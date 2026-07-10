package remote

import (
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/jlaffaye/ftp"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type AuthSecret struct {
	Password     string `json:"password,omitempty"`
	PrivateKey   string `json:"privateKeyPem,omitempty"`
	Passphrase   string `json:"passphrase,omitempty"`
	PublicKey    string `json:"publicKey,omitempty"`
}

type FileTarget struct {
	Protocol     string
	Host         string
	Port         int
	Username     string
	RemoteRoot   string
	IncludePaths []string
	AuthMode     string
	Secret       AuthSecret
}

func TestFileTarget(t FileTarget) (string, error) {
	switch t.Protocol {
	case "sftp", "rsync":
		return testSSHList(t)
	case "ftp", "ftps":
		return testFTP(t)
	default:
		return "", fmt.Errorf("unsupported protocol %q", t.Protocol)
	}
}

func testSSHList(t FileTarget) (string, error) {
	client, err := DialSSH(t.Host, t.Port, t.Username, t.AuthMode, t.Secret)
	if err != nil {
		return "", err
	}
	defer client.Close()

	if t.Protocol == "rsync" {
		session, err := client.NewSession()
		if err != nil {
			return "", err
		}
		defer session.Close()
		out, err := session.CombinedOutput("command -v rsync >/dev/null && echo rsync-ok || echo rsync-missing")
		if err != nil {
			return "", fmt.Errorf("ssh ok but remote check failed: %w (%s)", err, string(out))
		}
		msg := string(out)
		if len(msg) == 0 {
			msg = "ssh ok"
		}
		return "SSH OK — " + msg, nil
	}

	sc, err := sftp.NewClient(client)
	if err != nil {
		return "", fmt.Errorf("sftp: %w", err)
	}
	defer sc.Close()
	root := t.RemoteRoot
	if root == "" {
		root, _ = sc.Getwd()
		if root == "" {
			root = "/"
		}
	}
	fi, err := sc.Stat(root)
	if err != nil {
		return "", fmt.Errorf("remote path %q: %w", root, err)
	}
	kind := "file"
	if fi.IsDir() {
		kind = "directory"
	}
	return fmt.Sprintf("SFTP OK — %s is a %s", root, kind), nil
}

func testFTP(t FileTarget) (string, error) {
	addr := net.JoinHostPort(t.Host, fmt.Sprintf("%d", t.Port))
	opts := []ftp.DialOption{ftp.DialWithTimeout(15 * time.Second)}
	if t.Protocol == "ftps" {
		opts = append(opts, ftp.DialWithExplicitTLS(nil))
	}
	c, err := ftp.Dial(addr, opts...)
	if err != nil {
		return "", err
	}
	defer c.Quit()
	if err := c.Login(t.Username, t.Secret.Password); err != nil {
		return "", fmt.Errorf("login: %w", err)
	}
	if t.RemoteRoot != "" && t.RemoteRoot != "/" {
		if err := c.ChangeDir(t.RemoteRoot); err != nil {
			return "", fmt.Errorf("path %q: %w", t.RemoteRoot, err)
		}
	}
	return fmt.Sprintf("FTP OK — logged in, path %s reachable", t.RemoteRoot), nil
}

func DialSSH(host string, port int, user, authMode string, secret AuthSecret) (*ssh.Client, error) {
	var auth []ssh.AuthMethod
	switch authMode {
	case "password":
		auth = append(auth, ssh.Password(secret.Password))
	case "key":
		var signer ssh.Signer
		var err error
		if secret.Passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(secret.PrivateKey), []byte(secret.Passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey([]byte(secret.PrivateKey))
		}
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
		auth = append(auth, ssh.PublicKeys(signer))
	default:
		return nil, fmt.Errorf("unknown auth mode %q", authMode)
	}
	cfg := &ssh.ClientConfig{
		User:            user,
		Auth:            auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // lab appliance; document for production
		Timeout:         15 * time.Second,
	}
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	return ssh.Dial("tcp", addr, cfg)
}

func MarshalSecret(s AuthSecret) ([]byte, error) {
	return json.Marshal(s)
}

func UnmarshalSecret(b []byte) (AuthSecret, error) {
	var s AuthSecret
	if len(b) == 0 {
		return s, nil
	}
	err := json.Unmarshal(b, &s)
	return s, err
}
