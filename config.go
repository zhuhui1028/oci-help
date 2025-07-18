package main

import (
	"errors"
	"flag"
	"fmt"

	"gopkg.in/ini.v1"
)

// 全局配置常量
const (
	defConfigFilePath = "./oci-help.ini"
	IPsFilePrefix     = "IPs"
)

// 全局配置变量
var (
	configFilePath      string
	proxy               string
	token               string
	chat_id             string
	cmd                 string
	sendMessageUrl      string
	editMessageUrl      string
	EACH                bool
	oracleSections      []*ini.Section
	oracleSection       *ini.Section
	oracleSectionName   string
	instanceBaseSection *ini.Section
	oracle              Oracle
	instance            Instance
)

// Oracle 账号配置结构体
type Oracle struct {
	User         string `ini:"user"`
	Fingerprint  string `ini:"fingerprint"`
	Tenancy      string `ini:"tenancy"`
	Region       string `ini:"region"`
	Key_file     string `ini:"key_file"`
	Key_password string `ini:"key_password"`
}

// 实例配置结构体
type Instance struct {
	AvailabilityDomain     string  `ini:"availabilityDomain"`
	SSH_Public_Key         string  `ini:"ssh_authorized_key"`
	VcnDisplayName         string  `ini:"vcnDisplayName"`
	SubnetDisplayName      string  `ini:"subnetDisplayName"`
	Shape                  string  `ini:"shape"`
	OperatingSystem        string  `ini:"OperatingSystem"`
	OperatingSystemVersion string  `ini:"OperatingSystemVersion"`
	InstanceDisplayName    string  `ini:"instanceDisplayName"`
	Ocpus                  float32 `ini:"cpus"`
	MemoryInGBs            float32 `ini:"memoryInGBs"`
	Burstable              string  `ini:"burstable"`
	BootVolumeSizeInGBs    int64   `ini:"bootVolumeSizeInGBs"`
	Sum                    int32   `ini:"sum"`
	Each                   int32   `ini:"each"`
	Retry                  int32   `ini:"retry"`
	CloudInit              string  `ini:"cloud-init"`
	MinTime                int32   `ini:"minTime"`
	MaxTime                int32   `ini:"maxTime"`
}

// init 在 main 函数之前运行，用于解析命令行参数
func init() {
	flag.StringVar(&configFilePath, "config", defConfigFilePath, "配置文件路径")
	flag.StringVar(&configFilePath, "c", defConfigFilePath, "配置文件路径 (简写)")
	flag.Parse()
}

// loadConfig 加载并解析 oci-help.ini 配置文件
func loadConfig(path string) error {
	cfg, err := ini.Load(path)
	if err != nil {
		return fmt.Errorf("无法加载配置文件: %v", err)
	}

	defSec := cfg.Section(ini.DefaultSection)
	proxy = defSec.Key("proxy").Value()
	token = defSec.Key("token").Value()
	chat_id = defSec.Key("chat_id").Value()
	cmd = defSec.Key("cmd").Value()
	if defSec.HasKey("EACH") {
		EACH, _ = defSec.Key("EACH").Bool()
	} else {
		EACH = true
	}

	sendMessageUrl = "https://api.telegram.org/bot" + token + "/sendMessage"
	editMessageUrl = "https://api.telegram.org/bot" + token + "/editMessageText"

	oracleSections = []*ini.Section{}
	for _, sec := range cfg.Sections() {
		if isOracleSection(sec) {
			oracleSections = append(oracleSections, sec)
		}
	}

	if len(oracleSections) == 0 {
		return errors.New("在配置文件中未找到格式正确的甲骨文账号信息")
	}

	instanceBaseSection = cfg.Section("INSTANCE")

	return nil
}

// isOracleSection 检查一个 INI section 是否是有效的 Oracle 账号配置
func isOracleSection(sec *ini.Section) bool {
	return sec.HasKey("user") &&
		sec.HasKey("fingerprint") &&
		sec.HasKey("tenancy") &&
		sec.HasKey("region") &&
		sec.HasKey("key_file") &&
		len(sec.ParentKeys()) == 0
}
