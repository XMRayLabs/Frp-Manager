// Package websiteauth defines the optional HTTP authentication boundary for TCP websites.
package websiteauth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"unicode"

	v1 "github.com/fatedier/frp/pkg/config/v1"
	"golang.org/x/crypto/bcrypt"
)

const PluginType = "fm_http_auth"
const MaxAccounts = 32

type Account struct {
	Username     string `json:"username"`
	Password     string `json:"password,omitempty"`
	PasswordHash string `json:"passwordHash,omitempty"`
}
type Options struct {
	Type      string    `json:"type"`
	LocalAddr string    `json:"localAddr"`
	Accounts  []Account `json:"accounts"`
}

func (o *Options) Complete() {}
func (o *Options) Clone() v1.ClientPluginOptions {
	copy := *o
	copy.Accounts = append([]Account(nil), o.Accounts...)
	return &copy
}
func Decode(raw []byte) (*Options, error) {
	var o Options
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&o); err != nil {
		return nil, fmt.Errorf("网站认证配置格式不正确")
	}
	if err := o.Normalize(); err != nil {
		return nil, err
	}
	return &o, nil
}

// Normalize hashes newly entered passwords. Existing hashes survive edits; plaintext never persists.
func (o *Options) Normalize() error {
	if o.Type != PluginType {
		return fmt.Errorf("网站认证插件类型不正确")
	}
	host, port, err := net.SplitHostPort(o.LocalAddr)
	n, e := strconv.Atoi(port)
	if err != nil || e != nil || n < 1 || n > 65535 || strings.TrimSpace(host) == "" || strings.ContainsAny(host, "/\\?#@ \t\r\n") {
		return fmt.Errorf("网站目标必须为有效的主机:端口，IPv6 请使用方括号")
	}
	if len(o.Accounts) < 1 || len(o.Accounts) > MaxAccounts {
		return fmt.Errorf("网站认证需要 1 至 32 组账号，关闭认证请使用开关")
	}
	seen := map[string]bool{}
	for i := range o.Accounts {
		a := &o.Accounts[i]
		if len(a.Username) < 1 || len(a.Username) > 64 || strings.Contains(a.Username, ":") || strings.IndexFunc(a.Username, unicode.IsControl) >= 0 || strings.TrimSpace(a.Username) != a.Username || seen[a.Username] {
			return fmt.Errorf("网站用户名不能为空、重复或包含冒号及控制字符（最多 64 字节）")
		}
		seen[a.Username] = true
		if a.Password != "" {
			if len(a.Password) < 8 || len(a.Password) > 72 {
				return fmt.Errorf("网站密码长度应为 8 至 72 字节")
			}
			hash, err := bcrypt.GenerateFromPassword([]byte(a.Password), bcrypt.DefaultCost)
			if err != nil {
				return fmt.Errorf("网站密码处理失败")
			}
			a.PasswordHash = string(hash)
			a.Password = ""
		}
		cost, err := bcrypt.Cost([]byte(a.PasswordHash))
		if err != nil || cost != bcrypt.DefaultCost {
			return fmt.Errorf("请为网站账号填写密码（8 至 72 字节）")
		}
	}
	return nil
}

// SupportsVersion checks the product core version, not the embedded FRP version.
func SupportsVersion(version string) bool {
	parts := strings.Split(strings.TrimPrefix(version, "v"), ".")
	if len(parts) != 3 {
		return false
	}
	values := [3]int{}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return false
		}
		values[i] = n
	}
	return values[0] > 1 || (values[0] == 1 && (values[1] > 1 || (values[1] == 1 && values[2] >= 6)))
}
