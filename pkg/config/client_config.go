package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	clientConfigDirName  = "rm-aurora"
	clientConfigFileName = "ui-config.json"
)

// resolveClientConfigPath 解析客户端配置文件绝对路径。
// What: 将配置统一写入用户目录下的 config 路径。
// Why: 避免依赖当前工作目录，防止不同启动方式导致配置丢失。
func resolveClientConfigPath() (string, error) {
	baseDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir failed: %w", err)
	}
	return filepath.Join(baseDir, clientConfigDirName, clientConfigFileName), nil
}

// SaveClientConfigJSON 保存前端透传的 JSON 配置。
// What: 对 JSON 先做结构合法性校验，再原子写入本地文件。
// Why: 防止脏配置覆盖导致下次启动解析失败，且避免并发写入产生半文件。
func SaveClientConfigJSON(raw string) error {
	if !json.Valid([]byte(raw)) {
		return fmt.Errorf("invalid config json payload")
	}

	configPath, err := resolveClientConfigPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return fmt.Errorf("create config dir failed: %w", err)
	}

	tempPath := configPath + ".tmp"
	if err := os.WriteFile(tempPath, []byte(raw), 0o644); err != nil {
		return fmt.Errorf("write temp config failed: %w", err)
	}
	if err := os.Rename(tempPath, configPath); err != nil {
		return fmt.Errorf("replace config file failed: %w", err)
	}
	return nil
}

// LoadClientConfigJSON 读取本地配置 JSON。
// What: 若本地不存在配置则返回空对象 JSON。
// Why: 让前端可统一按 JSON 流程解析，避免空字符串引发异常分支。
func LoadClientConfigJSON() (string, error) {
	configPath, err := resolveClientConfigPath()
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "{}", nil
		}
		return "", fmt.Errorf("read config failed: %w", err)
	}

	if !json.Valid(data) {
		// What: 本地文件损坏时回退到空对象。
		// Why: 防止前端因历史坏文件启动失败，同时保留修复入口。
		return "{}", nil
	}
	return string(data), nil
}
