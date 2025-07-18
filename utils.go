package main

import (
	"bytes"
	"fmt"
	"math/rand"
	"os/exec"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
)

// printf 在标准输出前添加时间戳
func printf(format string, a ...interface{}) {
	fmt.Printf("%s ", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Printf(format, a...)
}

// printlnErr 以红色打印格式化的错误消息
func printlnErr(desc, detail string) {
	fmt.Printf("\033[1;31mError: %s. %s\033[0m\n", desc, detail)
}

// sleepRandomSecond 在指定的最小和最大值之间随机休眠一段时间（单位：秒）
func sleepRandomSecond(min, max int32) {
	var second int32
	if min <= 0 || max <= 0 {
		second = 1
	} else if min >= max {
		second = max
	} else {
		// rand.Int31n(n) 返回 [0, n) 的随机整数
		second = rand.Int31n(max-min) + min
	}
	printf("Sleep %d Second...\n", second)
	time.Sleep(time.Duration(second) * time.Second)
}

// fmtDuration 将 time.Duration 类型转换为 "X 天 X 时 X 分 X 秒" 的可读格式
func fmtDuration(d time.Duration) string {
	if d.Seconds() < 1 {
		return "< 1 秒"
	}
	var buffer bytes.Buffer

	days := int(d / (time.Hour * 24))
	hours := int((d % (time.Hour * 24)).Hours())
	minutes := int((d % time.Hour).Minutes())
	seconds := int((d % time.Minute).Seconds())

	if days > 0 {
		buffer.WriteString(fmt.Sprintf("%d 天 ", days))
	}
	if hours > 0 {
		buffer.WriteString(fmt.Sprintf("%d 时 ", hours))
	}
	if minutes > 0 {
		buffer.WriteString(fmt.Sprintf("%d 分 ", minutes))
	}
	if seconds > 0 {
		buffer.WriteString(fmt.Sprintf("%d 秒", seconds))
	}
	return buffer.String()
}

// getInstanceState 将实例的生命周期状态转换为中文描述
func getInstanceState(state core.InstanceLifecycleStateEnum) string {
	var friendlyState string
	switch state {
	case core.InstanceLifecycleStateMoving:
		friendlyState = "正在移动"
	case core.InstanceLifecycleStateProvisioning:
		friendlyState = "正在预配"
	case core.InstanceLifecycleStateRunning:
		friendlyState = "正在运行"
	case core.InstanceLifecycleStateStarting:
		friendlyState = "正在启动"
	case core.InstanceLifecycleStateStopping:
		friendlyState = "正在停止"
	case core.InstanceLifecycleStateStopped:
		friendlyState = "已停止　" // 使用全角空格为了对齐
	case core.InstanceLifecycleStateTerminating:
		friendlyState = "正在终止"
	case core.InstanceLifecycleStateTerminated:
		friendlyState = "已终止　" // 使用全角空格为了对齐
	default:
		friendlyState = string(state)
	}
	return friendlyState
}

// getBootVolumeState 将引导卷的生命周期状态转换为中文描述
func getBootVolumeState(state core.BootVolumeLifecycleStateEnum) string {
	var friendlyState string
	switch state {
	case core.BootVolumeLifecycleStateProvisioning:
		friendlyState = "正在预配"
	case core.BootVolumeLifecycleStateRestoring:
		friendlyState = "正在恢复"
	case core.BootVolumeLifecycleStateAvailable:
		friendlyState = "可用　　"
	case core.BootVolumeLifecycleStateTerminating:
		friendlyState = "正在终止"
	case core.BootVolumeLifecycleStateTerminated:
		friendlyState = "已终止　"
	case core.BootVolumeLifecycleStateFaulty:
		friendlyState = "故障　　"
	default:
		friendlyState = string(state)
	}
	return friendlyState
}

// command 执行一个外部shell命令
func command(cmd string) {
	res := strings.Fields(cmd)
	if len(res) > 0 {
		fmt.Println("执行命令:", strings.Join(res, " "))
		name := res[0]
		arg := res[1:]
		// CombinedOutput 会同时捕获标准输出和标准错误
		out, err := exec.Command(name, arg...).CombinedOutput()
		if err != nil {
			fmt.Printf("命令执行失败: %v\n", err)
		}
		fmt.Println(string(out))
	}
}

// getCustomRequestMetadataWithRetryPolicy 创建一个包含自定义重试策略的请求元数据
func getCustomRequestMetadataWithRetryPolicy() common.RequestMetadata {
	return common.RequestMetadata{
		RetryPolicy: getCustomRetryPolicy(),
	}
}

// getCustomRetryPolicy 定义了一个简单的重试策略：尝试3次，并在所有非2xx状态码时重试
func getCustomRetryPolicy() *common.RetryPolicy {
	// 定义重试次数
	attempts := uint(3)

	// 定义重试条件：任何非成功状态码（即非 200-299）都应该重试
	retryOnAllNon200ResponseCodes := func(r common.OCIOperationResponse) bool {
		// 如果没有错误且状态码在200-299之间，则不重试 (返回 false)
		if r.Error == nil && 199 < r.Response.HTTPResponse().StatusCode && r.Response.HTTPResponse().StatusCode < 300 {
			return false
		}
		// 否则，重试 (返回 true)
		return true
	}

	// 创建并返回策略
	policy := common.NewRetryPolicyWithOptions(
		common.WithMaximumNumberAttempts(attempts),
		common.WithShouldRetryOperation(retryOnAllNon200ResponseCodes),
	)
	return &policy
}

// getCustomRequestMetadataWithCustomizedRetryPolicy 创建一个包含自定义重试函数的请求元数据
func getCustomRequestMetadataWithCustomizedRetryPolicy(shouldRetryFunc func(r common.OCIOperationResponse) bool) common.RequestMetadata {
	return common.RequestMetadata{
		RetryPolicy: getCustomizedRetryPolicy(shouldRetryFunc),
	}
}

// getCustomizedRetryPolicy 创建一个使用自定义重试函数的策略
func getCustomizedRetryPolicy(shouldRetryFunc func(r common.OCIOperationResponse) bool) *common.RetryPolicy {
	policy := common.NewRetryPolicyWithOptions(
		common.WithShouldRetryOperation(shouldRetryFunc),
	)
	return &policy
}
