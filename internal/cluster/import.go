package cluster

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const maxConfigSize = 4 << 20

type entry struct {
	profile Profile
	config  *clientcmdapi.Config
}

func loadEntries(path string) ([]entry, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("解析配置路径: %w", err)
	}
	file, err := os.Open(absolute)
	if err != nil {
		return nil, fmt.Errorf("读取 kubeconfig: %w", err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxConfigSize+1))
	if err != nil {
		return nil, fmt.Errorf("读取 kubeconfig: %w", err)
	}
	if len(data) > maxConfigSize {
		return nil, errors.New("kubeconfig 超过 4 MiB 限制")
	}
	config, err := clientcmd.Load(data)
	if err != nil {
		// Parser errors can contain credential-bearing source lines.
		return nil, errors.New("无法解析 kubeconfig，请检查 YAML 格式和配置结构")
	}
	if len(config.Contexts) == 0 {
		return nil, errors.New("kubeconfig 中没有 Context")
	}
	for _, cluster := range config.Clusters {
		cluster.LocationOfOrigin = absolute
	}
	for _, auth := range config.AuthInfos {
		auth.LocationOfOrigin = absolute
	}
	if err := clientcmd.ResolveLocalPaths(config); err != nil {
		return nil, errors.New("无法解析 kubeconfig 中的证书或认证插件路径")
	}
	entries := make([]entry, 0, len(config.Contexts))
	for name, context := range config.Contexts {
		profile, err := describeProfile(absolute, name, context, config, data)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry{profile: profile, config: config})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].profile.Context < entries[j].profile.Context })
	return entries, nil
}

func describeProfile(path, name string, context *clientcmdapi.Context, config *clientcmdapi.Config, data []byte) (Profile, error) {
	cluster, ok := config.Clusters[context.Cluster]
	if !ok {
		return Profile{}, fmt.Errorf("Context %q 引用了不存在的集群", name)
	}
	server, err := url.Parse(cluster.Server)
	if err != nil || server.Host == "" || (server.Scheme != "https" && server.Scheme != "http") || server.User != nil || server.RawQuery != "" || server.Fragment != "" {
		return Profile{}, fmt.Errorf("Context %q 的 API Server 地址无效", name)
	}
	hash := sha256.New()
	_, _ = hash.Write([]byte(path + "\x00" + name + "\x00"))
	_, _ = hash.Write(data)
	profile := Profile{
		ID: hex.EncodeToString(hash.Sum(nil))[:24], Context: name, Cluster: context.Cluster,
		Server: server.String(), Namespace: context.Namespace, Source: path,
		InsecureTLS: cluster.InsecureSkipTLSVerify,
	}
	if profile.Namespace == "" {
		profile.Namespace = "default"
	}
	if auth := config.AuthInfos[context.AuthInfo]; auth != nil && auth.Exec != nil {
		profile.ExecCommand = auth.Exec.Command
	}
	return profile, nil
}
