package main

import (
	"log"
	"math/rand"
	"time"
)

func main() {
	// 初始化随机数种子
	rand.Seed(time.Now().UnixNano())

	// 加载并解析配置文件
	err := loadConfig(defConfigFilePath)
	if err != nil {
		log.Fatalf("错误: 无法加载配置文件。%v", err)
	}

	// 如果配置文件中定义了多个账号，则让用户选择一个
	// 如果只有一个账号，则直接使用
	if len(oracleSections) == 1 {
		oracleSection = oracleSections[0]
		err = initOCIClient(oracleSection)
		if err != nil {
			return // initOCIClient 内部会打印错误
		}
		showMainMenu()
	} else if len(oracleSections) > 1 {
		listOracleAccounts()
	} else {
		log.Fatalf("错误: 在 %s 中未找到有效的甲骨文账号配置。", defConfigFilePath)
	}
}
