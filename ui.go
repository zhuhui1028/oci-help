package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/oracle/oci-go-sdk/v65/identity"
	"gopkg.in/ini.v1"
)

// --- 辅助函数 ---

func printMenuTitle(title string) {
	fmt.Print("\033[H\033[2J")
	fmt.Printf("\n\033[1;32m--- %s ---\033[0m\n", title)
	fmt.Printf("\033[1;36m当前账号: %s\033[0m\n\n", oracleSection.Name())
}

func readInput() string {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	return strings.TrimSpace(scanner.Text())
}

func promptToContinue() {
	fmt.Print("\n按回车键继续...")
	readInput()
}

func handleActionError(err error, actionName string) bool {
	if err != nil {
		printlnErr(fmt.Sprintf("%s失败", actionName), err.Error())
		return false
	}
	fmt.Printf("%s操作已成功发起。\n", actionName)
	return true
}

// --- 主流程与菜单 ---

func listOracleAccounts() {
	for {
		fmt.Print("\033[H\033[2J")
		fmt.Printf("\n\033[1;32m--- 欢迎使用甲骨文实例管理工具 ---\033[0m\n\n")

		w := new(tabwriter.Writer)
		w.Init(os.Stdout, 0, 8, 2, '\t', 0)
		fmt.Fprintln(w, "序号\t账号")
		fmt.Fprintln(w, "--\t--")
		for i, section := range oracleSections {
			fmt.Fprintf(w, "%d\t%s\n", i+1, section.Name())
		}
		w.Flush()
		fmt.Println()

		fmt.Print("请输入账号序号, 或 'q' 退出, 'oci' 批量创建, 'ip' 批量导出IP: ")
		input := readInput()

		if strings.EqualFold(input, "q") {
			fmt.Println("感谢使用，再见！")
			os.Exit(0)
		}

		if strings.EqualFold(input, "oci") {
			multiBatchLaunchInstances()
			promptToContinue()
			continue
		} else if strings.EqualFold(input, "ip") {
			multiBatchListInstancesIp()
			promptToContinue()
			continue
		}

		index, err := strconv.Atoi(input)
		if err == nil && 0 < index && index <= len(oracleSections) {
			oracleSection = oracleSections[index-1]
			err := initOCIClient(oracleSection)
			if err != nil {
				printlnErr("初始化OCI客户端失败", err.Error())
				promptToContinue()
				continue
			}
			showMainMenu()
		} else {
			fmt.Printf("\033[1;31m错误! 请输入有效的序号。\033[0m\n")
			time.Sleep(1 * time.Second)
		}
	}
}

func showMainMenu() {
	for {
		printMenuTitle("主菜单")
		fmt.Println("1. 实例管理")
		fmt.Println("2. 引导卷管理")
		fmt.Println("3. 管理员 (IAM)")
		fmt.Println("4. 网络管理 (VCN与防火墙)")
		fmt.Println("\nb. 返回账号选择")
		fmt.Print("\n请输入操作序号: ")

		switch readInput() {
		case "1":
			showInstanceMenu()
		case "2":
			listBootVolumes()
		case "3":
			listAdmins()
		case "4":
			showNetworkMenu()
		case "b":
			return
		default:
			fmt.Println("\033[1;31m输入无效!\033[0m")
			time.Sleep(1 * time.Second)
		}
	}
}

// --- 实例管理 ---

func showInstanceMenu() {
	for {
		printMenuTitle("实例管理")
		fmt.Println("1. 查看所有实例")
		fmt.Println("2. 创建实例")
		fmt.Println("\nb. 返回主菜单")
		fmt.Print("\n请输入操作序号: ")

		switch readInput() {
		case "1":
			listInstances()
		case "2":
			listLaunchInstanceTemplates()
		case "b":
			return
		default:
			fmt.Println("\033[1;31m输入无效!\033[0m")
			time.Sleep(1 * time.Second)
		}
	}
}

func listInstances() {
	for {
		printMenuTitle("实例列表")
		fmt.Println("正在获取实例数据...")
		var instances []core.Instance
		var ins []core.Instance
		var nextPage *string
		var err error
		for {
			ins, nextPage, err = ListInstances(ctx, computeClient, nextPage)
			if err == nil {
				instances = append(instances, ins...)
			}
			if nextPage == nil || len(ins) == 0 {
				break
			}
		}

		if err != nil {
			printlnErr("获取实例失败", err.Error())
			promptToContinue()
			return
		}
		if len(instances) == 0 {
			fmt.Println("此账户下没有实例。")
			promptToContinue()
			return
		}

		w := new(tabwriter.Writer)
		w.Init(os.Stdout, 0, 8, 2, '\t', 0)
		fmt.Fprintln(w, "序号\t名称\t状态\t配置\t可用区")
		fmt.Fprintln(w, "--\t--\t--\t--\t---")
		for i, inst := range instances {
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n", i+1, *inst.DisplayName, getInstanceState(inst.LifecycleState), *inst.Shape, *inst.AvailabilityDomain)
		}
		w.Flush()
		fmt.Println(strings.Repeat("-", 60))

		fmt.Print("\n输入序号查看详情, 或 'b' 返回: ")
		input := readInput()
		if strings.EqualFold(input, "b") {
			return
		}

		index, err := strconv.Atoi(input)
		if err == nil && 0 < index && index <= len(instances) {
			instanceDetails(instances[index-1].Id)
		} else {
			fmt.Println("\033[1;31m输入无效!\033[0m")
			time.Sleep(1 * time.Second)
		}
	}
}

func instanceDetails(instanceId *string) {
	for {
		printMenuTitle("实例详细信息")
		fmt.Println("正在获取实例详细信息...")
		instance, err := getInstance(instanceId)
		if err != nil {
			printlnErr("获取实例详细信息失败", err.Error())
			promptToContinue()
			return
		}

		vnics, _ := getInstanceVnics(instance.Id)
		var primaryVnic core.Vnic
		var publicIPs, ipv6Addresses []string
		var subnetId string
		if len(vnics) > 0 {
			for _, vnic := range vnics {
				if vnic.IsPrimary != nil && *vnic.IsPrimary {
					primaryVnic = vnic
					break
				}
			}
			if primaryVnic.Id != nil {
				if primaryVnic.PublicIp != nil {
					publicIPs = append(publicIPs, *primaryVnic.PublicIp)
				}
				ipv6s, _ := ListIpv6s(primaryVnic.Id)
				for _, ipv6 := range ipv6s {
					ipv6Addresses = append(ipv6Addresses, *ipv6.IpAddress)
				}
				subnetId = *primaryVnic.SubnetId
			}
		}

		fmt.Printf("\033[1m%s\033[0m\n", *instance.DisplayName)
		fmt.Println(strings.Repeat("-", 50))
		fmt.Printf("%-12s: %s\n", "状态", getInstanceState(instance.LifecycleState))
		fmt.Printf("%-12s: %s\n", "开机时间", instance.TimeCreated.Format("2006-01-02 15:04:05"))
		fmt.Printf("%-12s: %s\n", "公共 IPv4", strings.Join(publicIPs, ", "))
		fmt.Printf("%-12s: %s\n", "公共 IPv6", strings.Join(ipv6Addresses, ", "))
		fmt.Printf("%-12s: %s\n", "可用区", *instance.AvailabilityDomain)
		fmt.Printf("%-12s: %s\n", "子网 OCID", subnetId)
		fmt.Printf("%-12s: %s\n", "配置", *instance.Shape)
		if instance.ShapeConfig != nil {
			fmt.Printf("%-12s: %g\n", "OCPU", *instance.ShapeConfig.Ocpus)
			fmt.Printf("%-12s: %g GB\n", "内存", *instance.ShapeConfig.MemoryInGBs)
		}
		fmt.Println(strings.Repeat("-", 50))

		fmt.Println("\n--- 操作菜单 ---")
		fmt.Println("1. 启动   2. 停止   3. 重启   4. 终止")
		fmt.Println("5. 管理 IPv6 地址")
		fmt.Println("6. 查看实例流量")
		fmt.Println("\nb. 返回实例列表")
		fmt.Print("\n请输入操作序号: ")

		input := readInput()
		switch input {
		case "1":
			_, err := instanceAction(instance.Id, core.InstanceActionActionStart)
			handleActionError(err, "启动")
		case "2":
			_, err := instanceAction(instance.Id, core.InstanceActionActionSoftstop)
			handleActionError(err, "停止")
		case "3":
			_, err := instanceAction(instance.Id, core.InstanceActionActionSoftreset)
			handleActionError(err, "重启")
		case "4":
			fmt.Print("确定终止实例？(输入 y 确认): ")
			if readInput() == "y" {
				err := terminateInstance(instance.Id)
				if handleActionError(err, "终止") {
					return
				}
			}
		case "5":
			if primaryVnic.Id != nil {
				manageIPv6(&primaryVnic)
			} else {
				fmt.Println("未找到主网卡，无法管理IPv6。")
			}
		case "6":
			showTrafficMenu(instance.Id)
		case "b":
			return
		default:
			fmt.Println("\033[1;31m输入无效。\033[0m")
		}
		if input != "b" {
			promptToContinue()
		}
	}
}

func manageIPv6(vnic *core.Vnic) {
	printMenuTitle("管理 IPv6 地址")

	ipv6s, err := ListIpv6s(vnic.Id)
	if err != nil {
		printlnErr("获取IPv6地址失败", err.Error())
		promptToContinue()
		return
	}

	if len(ipv6s) > 0 {
		fmt.Println("当前已分配的 IPv6 地址:")
		for _, ipv6 := range ipv6s {
			fmt.Printf("- %s\n", *ipv6.IpAddress)
		}
		fmt.Print("\n是否要删除现有IPv6并添加一个新的 (替换)？(y/n): ")
		if readInput() == "y" {
			fmt.Println("正在删除旧的 IPv6 地址...")
			err := DeleteIpv6(ipv6s[0].Id)
			if err != nil {
				printlnErr("删除失败", err.Error())
				promptToContinue()
				return
			}
			fmt.Println("删除成功，正在添加新地址...")
			time.Sleep(3 * time.Second)
		} else {
			return
		}
	}

	newIpv6, err := AddIpv6(vnic.Id)
	if err != nil {
		printlnErr("添加IPv6地址失败", err.Error())
	} else {
		fmt.Printf("成功添加新的 IPv6 地址: %s\n", *newIpv6.IpAddress)
	}
	promptToContinue()
}

func showTrafficMenu(instanceId *string) {
	for {
		printMenuTitle("查看实例流量")
		fmt.Println("1. 最近 24 小时")
		fmt.Println("2. 最近 7 天")
		fmt.Println("\nb. 返回")
		fmt.Print("\n请选择时间范围: ")

		input := readInput()
		now := time.Now().UTC()
		var startTime, endTime time.Time
		endTime = now

		switch input {
		case "1":
			startTime = now.Add(-24 * time.Hour)
		case "2":
			startTime = now.Add(-7 * 24 * time.Hour)
		case "b":
			return
		default:
			fmt.Println("\033[1;31m输入无效。\033[0m")
			time.Sleep(1 * time.Second)
			continue
		}

		fmt.Println("正在查询监控数据，请稍候...")
		bytesIn, bytesOut, err := GetInstanceNetworkMetrics(*instanceId, startTime.Format(time.RFC3339), endTime.Format(time.RFC3339))
		if err != nil {
			printlnErr("查询流量数据失败", err.Error())
			promptToContinue()
			continue
		}

		fmt.Printf("\n查询时间: %s 到 %s\n", startTime.Local().Format("2006-01-02 15:04"), endTime.Local().Format("2006-01-02 15:04"))
		fmt.Printf("总流入流量: %.2f GB\n", bytesIn/1024/1024/1024)
		fmt.Printf("总流出流量: %.2f GB\n", bytesOut/1024/1024/1024)
		promptToContinue()
	}
}

// --- 管理员 (IAM) 管理 ---

func listAdmins() {
	for {
		printMenuTitle("管理员列表")
		fmt.Println("正在获取管理员列表...")
		users, err := ListUsers()
		if err != nil {
			printlnErr("获取用户列表失败", err.Error())
			promptToContinue()
			return
		}

		w := new(tabwriter.Writer)
		w.Init(os.Stdout, 0, 8, 2, '\t', 0)
		fmt.Fprintln(w, "序号\t名称\t邮箱\t状态\t创建时间")
		fmt.Fprintln(w, "--\t--\t--\t--\t---")
		for i, user := range users {
			email := ""
			if user.Email != nil {
				email = *user.Email
			}
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n", i+1, *user.Name, email, user.LifecycleState, user.TimeCreated.Format("2006-01-02"))
		}
		w.Flush()

		fmt.Println("\n'n' - 新增管理员 | 输入序号查看详情和操作 | 'b' - 返回")
		fmt.Print("请输入: ")
		input := readInput()

		if strings.EqualFold(input, "b") {
			return
		}
		if strings.EqualFold(input, "n") {
			createAdmin()
			continue
		}

		index, err := strconv.Atoi(input)
		if err == nil && index > 0 && index <= len(users) {
			adminDetails(&users[index-1])
		} else {
			fmt.Println("\033[1;31m输入无效。\033[0m")
			time.Sleep(1 * time.Second)
		}
	}
}

func adminDetails(user *identity.User) {
	for {
		printMenuTitle(fmt.Sprintf("管理员详情: %s", *user.Name))

		email, desc := "", ""
		if user.Email != nil {
			email = *user.Email
		}
		if user.Description != nil {
			desc = *user.Description
		}

		fmt.Printf("%-15s: %s\n", "名称", *user.Name)
		fmt.Printf("%-15s: %s\n", "OCID", *user.Id)
		fmt.Printf("%-15s: %s\n", "描述", desc)
		fmt.Printf("%-15s: %s\n", "邮箱", email)
		fmt.Printf("%-15s: %s\n", "创建时间", user.TimeCreated.Format("2006-01-02 15:04:05"))
		fmt.Printf("%-15s: %s\n", "状态", user.LifecycleState)
		fmt.Println(strings.Repeat("-", 50))

		fmt.Println("1. 修改描述/邮箱")
		fmt.Println("2. 重置多因子认证 (MFA)")
		fmt.Println("3. \033[1;31m删除此管理员\033[0m")
		fmt.Println("\nb. 返回管理员列表")
		fmt.Print("\n请输入操作序号: ")

		switch readInput() {
		case "1":
			updateAdminInfo(user)
		case "2":
			resetAdminMFA(user)
		case "3":
			if deleteAdmin(user) {
				return
			}
		case "b":
			return
		default:
			fmt.Println("\033[1;31m输入无效。\033[0m")
			time.Sleep(1 * time.Second)
		}
		refreshedUser, err := GetUser(user.Id)
		if err == nil {
			user = &refreshedUser
		} else {
			printlnErr("刷新用户信息失败", err.Error())
			return
		}
	}
}

func createAdmin() {
	printMenuTitle("新增管理员")
	fmt.Print("请输入新管理员的名称 (唯一): ")
	name := readInput()
	fmt.Print("请输入描述信息: ")
	description := readInput()
	fmt.Print("请输入邮箱地址: ")
	email := readInput()

	if name == "" || email == "" {
		fmt.Println("管理员名称和邮箱不能为空。")
		promptToContinue()
		return
	}

	fmt.Println("正在创建用户...")
	user, err := CreateUser(name, description, email)
	if err != nil {
		printlnErr("创建用户失败", err.Error())
		promptToContinue()
		return
	}
	fmt.Printf("用户 '%s' 创建成功！\n", *user.Name)

	fmt.Println("正在将用户添加到 'Administrators' 组...")
	err = AddUserToAdminGroup(user.Id)
	if err != nil {
		printlnErr("添加至管理员组失败", err.Error())
		fmt.Println("请注意：用户已创建，但需要手动将其加入 'Administrators' 组以获取完整权限。")
	} else {
		fmt.Println("管理员创建并授权成功！")
	}
	promptToContinue()
}

func updateAdminInfo(user *identity.User) {
	currentDesc := ""
	if user.Description != nil {
		currentDesc = *user.Description
	}
	fmt.Printf("当前描述: %s\n", currentDesc)
	fmt.Print("输入新描述 (留空则不修改): ")
	newDesc := readInput()
	if newDesc == "" {
		newDesc = currentDesc
	}

	currentEmail := ""
	if user.Email != nil {
		currentEmail = *user.Email
	}
	fmt.Printf("当前邮箱: %s\n", currentEmail)
	fmt.Print("输入新邮箱 (留空则不修改): ")
	newEmail := readInput()
	if newEmail == "" {
		newEmail = currentEmail
	}

	_, err := UpdateUser(user.Id, &newDesc, &newEmail)
	if err != nil {
		printlnErr("更新信息失败", err.Error())
	} else {
		fmt.Println("管理员信息更新成功！")
	}
	promptToContinue()
}

func resetAdminMFA(user *identity.User) {
	fmt.Printf("\033[1;31m警告：这将删除用户 '%s' 的所有多因子认证设备，用户将可以用密码直接登录。\033[0m\n", *user.Name)
	fmt.Print("请输入 'yes' 确认重置: ")
	if readInput() != "yes" {
		fmt.Println("操作已取消。")
		promptToContinue()
		return
	}
	err := ResetMFA(user.Id)
	if err != nil {
		printlnErr("重置MFA失败", err.Error())
	} else {
		fmt.Println("用户MFA已成功重置。")
	}
	promptToContinue()
}

func deleteAdmin(user *identity.User) bool {
	fmt.Printf("\033[1;31m警告：这将永久删除用户 '%s'！此操作无法撤销。\033[0m\n", *user.Name)
	fmt.Print("请输入 'yes' 确认删除: ")
	if readInput() != "yes" {
		fmt.Println("操作已取消。")
		promptToContinue()
		return false
	}

	err := DeleteUser(user.Id)
	if err != nil {
		printlnErr("删除用户失败", err.Error())
		promptToContinue()
		return false
	}

	fmt.Printf("用户 '%s' 已成功删除。\n", *user.Name)
	promptToContinue()
	return true
}

// --- 网络管理 ---

func showNetworkMenu() {
	printMenuTitle("网络管理")
	fmt.Println("正在获取VCN列表...")
	vcns, err := listVcns(ctx, networkClient)
	if err != nil {
		printlnErr("获取VCN列表失败", err.Error())
		promptToContinue()
		return
	}

	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 8, 2, '\t', 0)
	fmt.Fprintln(w, "序号\tVCN 名称\tCIDR Block\t状态")
	fmt.Fprintln(w, "--\t--\t--\t--")
	for i, vcn := range vcns {
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", i+1, *vcn.DisplayName, *vcn.CidrBlock, vcn.LifecycleState)
	}
	w.Flush()

	fmt.Print("\n输入VCN序号可查看其防火墙规则 (或 'b' 返回): ")
	input := readInput()
	if strings.EqualFold(input, "b") {
		return
	}

	index, err := strconv.Atoi(input)
	if err != nil || index < 1 || index > len(vcns) {
		fmt.Println("\033[1;31m输入无效。\033[0m")
		promptToContinue()
		return
	}
	vcn := vcns[index-1]

	if vcn.DefaultSecurityListId != nil && *vcn.DefaultSecurityListId != "" {
		showSecurityListDetails(vcn.DefaultSecurityListId)
	} else {
		fmt.Println("该VCN没有默认安全列表。")
	}
	promptToContinue()
}

func showSecurityListDetails(securityListId *string) {
	sl, err := GetSecurityList(securityListId)
	if err != nil {
		printlnErr("获取安全列表失败", err.Error())
		return
	}

	printMenuTitle(fmt.Sprintf("防火墙规则 for %s", *sl.DisplayName))
	fmt.Println("--- 入站规则 (Ingress) ---")
	for _, rule := range sl.IngressSecurityRules {
		desc := ""
		if rule.Description != nil {
			desc = *rule.Description
		}
		fmt.Printf("源: %-18s 协议: %-10s 描述: %s\n", *rule.Source, *rule.Protocol, desc)
	}

	fmt.Println("\n--- 出站规则 (Egress) ---")
	for _, rule := range sl.EgressSecurityRules {
		desc := ""
		if rule.Description != nil {
			desc = *rule.Description
		}
		fmt.Printf("目标: %-18s 协议: %-10s 描述: %s\n", *rule.Destination, *rule.Protocol, desc)
	}
}

// --- 引导卷管理 ---

func listBootVolumes() {
	printMenuTitle("引导卷管理")
	var bootVolumes []core.BootVolume
	var wg sync.WaitGroup
	availabilityDomains, err := ListAvailabilityDomains()
	if err != nil {
		printlnErr("获取可用域失败", err.Error())
		promptToContinue()
		return
	}
	for _, ad := range availabilityDomains {
		wg.Add(1)
		go func(adName *string) {
			defer wg.Done()
			volumes, err := getBootVolumes(adName)
			if err != nil {
				printlnErr("获取引导卷失败", err.Error())
			} else {
				bootVolumes = append(bootVolumes, volumes...)
			}
		}(ad.Name)
	}
	wg.Wait()

	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 8, 2, '\t', 0)
	fmt.Fprintln(w, "序号\t名称\t状态\t大小(GB)")
	fmt.Fprintln(w, "--\t--\t--\t---")
	for i, volume := range bootVolumes {
		fmt.Fprintf(w, "%d\t%s\t%s\t%d\n", i+1, *volume.DisplayName, getBootVolumeState(volume.LifecycleState), *volume.SizeInGBs)
	}
	w.Flush()
	fmt.Println()

	fmt.Print("输入序号查看详情 (或 'b' 返回): ")
	input := readInput()
	if strings.EqualFold(input, "b") {
		return
	}

	index, err := strconv.Atoi(input)
	if err == nil && 0 < index && index <= len(bootVolumes) {
		bootvolumeDetails(bootVolumes[index-1].Id)
	} else {
		fmt.Println("\033[1;31m输入无效。\033[0m")
		time.Sleep(1 * time.Second)
	}
}

func bootvolumeDetails(bootVolumeId *string) {
	for {
		printMenuTitle("引导卷详细信息")
		bootVolume, err := getBootVolume(bootVolumeId)
		if err != nil {
			printlnErr("获取引导卷详细信息失败", err.Error())
			promptToContinue()
			return
		}

		attachments, _ := listBootVolumeAttachments(bootVolume.AvailabilityDomain, bootVolume.CompartmentId, bootVolume.Id)
		var attachIns []string
		for _, attachment := range attachments {
			ins, err := getInstance(attachment.InstanceId)
			if err == nil {
				attachIns = append(attachIns, *ins.DisplayName)
			}
		}

		var performance string
		if bootVolume.VpusPerGB != nil {
			switch *bootVolume.VpusPerGB {
			case 10:
				performance = fmt.Sprintf("均衡 (VPU:%d)", *bootVolume.VpusPerGB)
			case 20:
				performance = fmt.Sprintf("性能较高 (VPU:%d)", *bootVolume.VpusPerGB)
			default:
				performance = fmt.Sprintf("UHP (VPU:%d)", *bootVolume.VpusPerGB)
			}
		}

		fmt.Printf("名称: %s\n", *bootVolume.DisplayName)
		fmt.Printf("状态: %s\n", getBootVolumeState(bootVolume.LifecycleState))
		fmt.Printf("大小(GB): %d\n", *bootVolume.SizeInGBs)
		fmt.Printf("性能: %s\n", performance)
		fmt.Printf("附加的实例: %s\n", strings.Join(attachIns, ", "))
		fmt.Println(strings.Repeat("-", 40))

		fmt.Println("1. 修改性能   2. 修改大小   3. 分离   4. 终止")
		fmt.Println("\nb. 返回")
		fmt.Print("\n请输入操作序号: ")

		input := readInput()
		switch input {
		case "1":
			fmt.Printf("修改引导卷性能, 请输入 (1: 均衡; 2: 性能较高): ")
			perfInput := readInput()
			var vpus int64
			if perfInput == "1" {
				vpus = 10
			} else if perfInput == "2" {
				vpus = 20
			} else {
				fmt.Println("输入无效。"); continue
			}
			_, err := updateBootVolume(bootVolume.Id, nil, &vpus)
			handleActionError(err, "修改性能")
		case "2":
			fmt.Printf("修改引导卷大小, 请输入 (例如修改为50GB, 输入50): ")
			sizeInGBs, err := strconv.ParseInt(readInput(), 10, 64)
			if err == nil && sizeInGBs > 0 {
				_, err := updateBootVolume(bootVolume.Id, &sizeInGBs, nil)
				handleActionError(err, "修改大小")
			} else {
				fmt.Println("输入无效。")
			}
		case "3":
			fmt.Printf("确定分离引导卷？(y/n): ")
			if readInput() == "y" {
				for _, attachment := range attachments {
					_, err := detachBootVolume(attachment.Id)
					handleActionError(err, "分离")
				}
			}
		case "4":
			fmt.Printf("确定终止引导卷？(y/n): ")
			if readInput() == "y" {
				_, err := deleteBootVolume(bootVolume.Id)
				if handleActionError(err, "终止") {
					return
				}
			}
		case "b":
			return
		default:
			fmt.Println("\033[1;31m输入无效。\033[0m")
		}
		if input != "b" {
			promptToContinue()
		}
	}
}

// --- 实例创建 ---

func listLaunchInstanceTemplates() {
	printMenuTitle("从模板创建实例")
	var instanceSections []*ini.Section
	instanceSections = append(instanceSections, instanceBaseSection.ChildSections()...)
	instanceSections = append(instanceSections, oracleSection.ChildSections()...)
	if len(instanceSections) == 0 {
		fmt.Println("未找到任何实例模版。")
		promptToContinue()
		return
	}

	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 8, 2, '\t', 0)
	fmt.Fprintln(w, "序号\t配置\tCPU\t内存(GB)")
	fmt.Fprintln(w, "--\t--\t---\t---")
	for i, instanceSec := range instanceSections {
		cpu := instanceSec.Key("cpus").Value()
		if cpu == "" {
			cpu = "-"
		}
		memory := instanceSec.Key("memoryInGBs").Value()
		if memory == "" {
			memory = "-"
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", i+1, instanceSec.Key("shape").Value(), cpu, memory)
	}
	w.Flush()
	fmt.Println()

	fmt.Print("请输入要创建的实例的序号 (或 'b' 返回): ")
	input := readInput()
	if strings.EqualFold(input, "b") {
		return
	}

	index, err := strconv.Atoi(input)
	if err == nil && 0 < index && index <= len(instanceSections) {
		instanceSection := instanceSections[index-1]
		instance = Instance{}
		instanceBaseSection.MapTo(&instance)
		instanceSection.MapTo(&instance)

		availabilityDomains, err := ListAvailabilityDomains()
		if err != nil {
			printlnErr("获取可用性域失败", err.Error())
		} else {
			LaunchInstances(availabilityDomains)
		}
	} else {
		fmt.Println("\033[1;31m输入无效。\033[0m")
	}
	promptToContinue()
}

// --- 批量操作 ---

func multiBatchLaunchInstances() {
	IPsFilePath := IPsFilePrefix + "-" + time.Now().Format("2006-01-02-150405.txt")
	for _, sec := range oracleSections {
		err := initOCIClient(sec)
		if err != nil {
			continue
		}
		batchLaunchInstances(sec)
		batchListInstancesIp(IPsFilePath, sec)
		command(cmd)
		sleepRandomSecond(5, 5)
	}
}

func batchLaunchInstances(oracleSec *ini.Section) {
	var instanceSections []*ini.Section
	instanceSections = append(instanceSections, instanceBaseSection.ChildSections()...)
	instanceSections = append(instanceSections, oracleSec.ChildSections()...)
	if len(instanceSections) == 0 {
		return
	}

	printf("\033[1;36m[%s] 开始批量创建\033[0m\n", oracleSectionName)
	sendMessage(fmt.Sprintf("[%s]", oracleSectionName), "开始批量创建")
	var totalSUM, totalNUM int32

	var wg sync.WaitGroup
	for _, instanceSec := range instanceSections {
		wg.Add(1)
		go func(sec *ini.Section) {
			defer wg.Done()
			localInstance := Instance{}
			instanceBaseSection.MapTo(&localInstance)
			sec.MapTo(&localInstance)

			availabilityDomains, err := ListAvailabilityDomains()
			if err != nil {
				printlnErr("获取可用性域失败", err.Error())
				return
			}
			sum, num := LaunchInstances(availabilityDomains)
			totalSUM += sum
			totalNUM += num
		}(instanceSec)
	}
	wg.Wait()

	text := fmt.Sprintf("结束创建。总计: %d, 成功: %d, 失败: %d", totalSUM, totalNUM, totalSUM-totalNUM)
	printf("\033[1;36m[%s] %s\033[0m\n", oracleSectionName, text)
	sendMessage(fmt.Sprintf("[%s]", oracleSectionName), text)
}

func multiBatchListInstancesIp() {
	IPsFilePath := IPsFilePrefix + "-" + time.Now().Format("2006-01-02-150405.txt")
	_, err := os.Stat(IPsFilePath)
	if err != nil && os.IsNotExist(err) {
		os.Create(IPsFilePath)
	}

	fmt.Printf("正在导出所有实例公共IP地址...\n")
	for _, sec := range oracleSections {
		err := initOCIClient(sec)
		if err != nil {
			continue
		}
		ListInstancesIPs(IPsFilePath, sec.Name())
	}
	fmt.Printf("导出完成，请查看文件 %s\n", IPsFilePath)
}

func batchListInstancesIp(filePath string, sec *ini.Section) {
	_, err := os.Stat(filePath)
	if err != nil && os.IsNotExist(err) {
		os.Create(filePath)
	}
	fmt.Printf("正在为账号 [%s] 导出实例公共IP地址...\n", sec.Name())
	ListInstancesIPs(filePath, sec.Name())
	fmt.Printf("导出完成，请查看文件 %s\n", filePath)
}

func ListInstancesIPs(filePath string, sectionName string) {
	var vnicAttachments []core.VnicAttachment
	var vas []core.VnicAttachment
	var nextPage *string
	var err error
	for {
		vas, nextPage, err = ListVnicAttachments(ctx, computeClient, nil, nextPage)
		if err == nil {
			vnicAttachments = append(vnicAttachments, vas...)
		}
		if nextPage == nil || len(vas) == 0 {
			break
		}
	}

	if err != nil {
		fmt.Printf("ListVnicAttachments Error: %s\n", err.Error())
		return
	}
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		fmt.Printf("打开文件失败, Error: %s\n", err.Error())
		return
	}
	defer file.Close()

	io.WriteString(file, "["+sectionName+"]\n")

	var wg sync.WaitGroup
	for _, vnicAttachment := range vnicAttachments {
		wg.Add(1)
		go func(va core.VnicAttachment) {
			defer wg.Done()
			vnic, err := GetVnic(ctx, networkClient, va.VnicId)
			if err != nil {
				fmt.Printf("IP地址获取失败, %s\n", err.Error())
				return
			}
			if vnic.PublicIp != nil && *vnic.PublicIp != "" {
				line := fmt.Sprintf("实例: %s, IP: %s\n", *vnic.DisplayName, *vnic.PublicIp)
				fmt.Print(line)
				io.WriteString(file, line)
			}
		}(vnicAttachment)
	}
	wg.Wait()
	io.WriteString(file, "\n")
}
