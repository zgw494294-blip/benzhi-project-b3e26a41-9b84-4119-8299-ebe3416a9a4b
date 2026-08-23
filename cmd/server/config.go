package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

const defaultAddress = "127.0.0.1:19081"

type config struct {
	Address   string
	DataPath  string
	SelfCheck bool
}

func parseConfig(arguments []string, getenv func(string) string) (config, error) {
	set := flag.NewFlagSet("server", flag.ContinueOnError)
	set.SetOutput(os.Stderr)
	address := set.String("addr", defaultAddress, "HTTP 监听地址")
	dataPath := set.String("data", "handover.db", "SQLite 数据文件路径")
	selfCheck := set.Bool("selfcheck", false, "运行有界业务自检后退出")
	if err := set.Parse(arguments); err != nil {
		return config{}, err
	}
	if set.NArg() != 0 {
		return config{}, fmt.Errorf("不支持的位置参数: %s", strings.Join(set.Args(), " "))
	}
	resolvedAddress := strings.TrimSpace(*address)
	if portValue := strings.TrimSpace(getenv("PORT")); portValue != "" {
		port, err := strconv.Atoi(portValue)
		if err != nil || port < 1 || port > 65535 {
			return config{}, errors.New("PORT 必须是 1 到 65535 之间的端口号")
		}
		resolvedAddress = net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	}
	if err := validateAddress(resolvedAddress); err != nil {
		return config{}, err
	}
	if strings.TrimSpace(*dataPath) == "" {
		return config{}, errors.New("-data 不能为空")
	}
	return config{Address: resolvedAddress, DataPath: strings.TrimSpace(*dataPath), SelfCheck: *selfCheck}, nil
}

func validateAddress(address string) error {
	host, portValue, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("-addr 必须是 host:port: %w", err)
	}
	port, err := strconv.Atoi(portValue)
	if err != nil || port < 1 || port > 65535 {
		return errors.New("-addr 端口必须是 1 到 65535 之间的数字")
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return errors.New("-addr 必须使用回环地址 127.0.0.1 或 ::1")
	}
	return nil
}
