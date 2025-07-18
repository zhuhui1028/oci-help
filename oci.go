package main

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/oracle/oci-go-sdk/v65/identity"
	"github.com/oracle/oci-go-sdk/v65/monitoring"
	"gopkg.in/ini.v1"
)

// 全局 OCI 客户端变量
var (
	provider         common.ConfigurationProvider
	computeClient    core.ComputeClient
	networkClient    core.VirtualNetworkClient
	storageClient    core.BlockstorageClient
	identityClient   identity.IdentityClient
	monitoringClient monitoring.MonitoringClient
	ctx              = context.Background()
)

// initOCIClient 根据指定的账号配置，初始化所有 OCI 服务客户端
func initOCIClient(oracleSec *ini.Section) (err error) {
	oracleSectionName = oracleSec.Name()
	oracle = Oracle{}
	err = oracleSec.MapTo(&oracle)
	if err != nil {
		printlnErr("解析账号相关参数失败", err.Error())
		return
	}

	provider, err = getProvider(oracle)
	if err != nil {
		printlnErr("获取 Provider 失败", err.Error())
		return
	}

	computeClient, err = core.NewComputeClientWithConfigurationProvider(provider)
	if err != nil {
		printlnErr("创建 ComputeClient 失败", err.Error()); return
	}
	setProxyOrNot(&computeClient.BaseClient)

	networkClient, err = core.NewVirtualNetworkClientWithConfigurationProvider(provider)
	if err != nil {
		printlnErr("创建 VirtualNetworkClient 失败", err.Error()); return
	}
	setProxyOrNot(&networkClient.BaseClient)

	storageClient, err = core.NewBlockstorageClientWithConfigurationProvider(provider)
	if err != nil {
		printlnErr("创建 BlockstorageClient 失败", err.Error()); return
	}
	setProxyOrNot(&storageClient.BaseClient)

	identityClient, err = identity.NewIdentityClientWithConfigurationProvider(provider)
	if err != nil {
		printlnErr("创建 IdentityClient 失败", err.Error()); return
	}
	setProxyOrNot(&identityClient.BaseClient)

	monitoringClient, err = monitoring.NewMonitoringClientWithConfigurationProvider(provider)
	if err != nil {
		printlnErr("创建 MonitoringClient 失败", err.Error()); return
	}
	setProxyOrNot(&monitoringClient.BaseClient)

	return
}

func getProvider(o Oracle) (common.ConfigurationProvider, error) {
	content, err := ioutil.ReadFile(o.Key_file)
	if err != nil {
		return nil, err
	}
	privateKey := string(content)
	privateKeyPassphrase := common.String(o.Key_password)
	return common.NewRawConfigurationProvider(o.Tenancy, o.User, o.Region, o.Fingerprint, privateKey, privateKeyPassphrase), nil
}

func setProxyOrNot(client *common.BaseClient) {
	if proxy != "" {
		proxyURL, err := url.Parse(proxy)
		if err != nil {
			printlnErr("代理URL解析失败", err.Error())
			return
		}
		client.HTTPClient = &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyURL(proxyURL),
			},
		}
	}
}

// --- IAM (管理员) 功能 ---

func ListUsers() ([]identity.User, error) {
	req := identity.ListUsersRequest{
		CompartmentId:   &oracle.Tenancy,
		RequestMetadata: getCustomRequestMetadataWithRetryPolicy(),
	}
	resp, err := identityClient.ListUsers(ctx, req)
	return resp.Items, err
}

func CreateUser(name, description, email string) (identity.User, error) {
	req := identity.CreateUserRequest{
		CreateUserDetails: identity.CreateUserDetails{
			CompartmentId: &oracle.Tenancy,
			Name:          &name,
			Description:   &description,
			Email:         &email,
		},
	}
	resp, err := identityClient.CreateUser(ctx, req)
	return resp.User, err
}

func AddUserToAdminGroup(userId *string) error {
	listGroupsReq := identity.ListGroupsRequest{
		CompartmentId: &oracle.Tenancy,
		Name:          common.String("Administrators"),
	}
	listGroupsResp, err := identityClient.ListGroups(ctx, listGroupsReq)
	if err != nil || len(listGroupsResp.Items) == 0 {
		return fmt.Errorf("找不到 Administrators 组: %v", err)
	}
	adminGroup := listGroupsResp.Items[0]

	addUserReq := identity.AddUserToGroupRequest{
		AddUserToGroupDetails: identity.AddUserToGroupDetails{
			UserId:  userId,
			GroupId: adminGroup.Id,
		},
	}
	_, err = identityClient.AddUserToGroup(ctx, addUserReq)
	return err
}

func DeleteUser(userId *string) error {
	req := identity.DeleteUserRequest{
		UserId: userId,
	}
	_, err := identityClient.DeleteUser(ctx, req)
	return err
}

func GetUser(userId *string) (identity.User, error) {
	req := identity.GetUserRequest{
		UserId: userId,
	}
	resp, err := identityClient.GetUser(ctx, req)
	return resp.User, err
}

func UpdateUser(userId *string, description, email *string) (identity.User, error) {
	req := identity.UpdateUserRequest{
		UserId: userId,
		UpdateUserDetails: identity.UpdateUserDetails{
			Description: description,
			Email:       email,
		},
	}
	resp, err := identityClient.UpdateUser(ctx, req)
	return resp.User, err
}

func ResetMFA(userId *string) error {
	listMfaDevicesReq := identity.ListMfaTotpDevicesRequest{
		UserId: userId,
	}
	resp, err := identityClient.ListMfaTotpDevices(ctx, listMfaDevicesReq)
	if err != nil {
		return fmt.Errorf("获取MFA设备列表失败: %v", err)
	}

	for _, device := range resp.Items {
		deleteReq := identity.DeleteMfaTotpDeviceRequest{
			UserId:          userId,
			MfaTotpDeviceId: device.Id,
		}
		_, err := identityClient.DeleteMfaTotpDevice(ctx, deleteReq)
		if err != nil {
			fmt.Printf("警告: 删除MFA设备 %s 失败: %v\n", *device.Id, err)
		}
	}
	return nil
}

// --- IPv6 和网络功能 ---

func GetSubnet(subnetId *string) (core.Subnet, error) {
	req := core.GetSubnetRequest{
		SubnetId: subnetId,
	}
	resp, err := networkClient.GetSubnet(ctx, req)
	return resp.Subnet, err
}

func ListIpv6s(vnicId *string) ([]core.Ipv6, error) {
	req := core.ListIpv6sRequest{
		VnicId: vnicId,
	}
	resp, err := networkClient.ListIpv6s(ctx, req)
	return resp.Items, err
}

func AddIpv6(vnicId *string) (core.Ipv6, error) {
	req := core.CreateIpv6Request{
		CreateIpv6Details: core.CreateIpv6Details{
			VnicId: vnicId,
		},
	}
	resp, err := networkClient.CreateIpv6(ctx, req)
	return resp.Ipv6, err
}

func DeleteIpv6(ipv6Id *string) error {
	req := core.DeleteIpv6Request{
		Ipv6Id: ipv6Id,
	}
	_, err := networkClient.DeleteIpv6(ctx, req)
	return err
}

func GetSecurityList(securityListId *string) (core.SecurityList, error) {
	req := core.GetSecurityListRequest{
		SecurityListId: securityListId,
	}
	resp, err := networkClient.GetSecurityList(ctx, req)
	return resp.SecurityList, err
}

func UpdateSecurityList(securityListId *string, ingressRules []core.IngressSecurityRule, egressRules []core.EgressSecurityRule) (core.SecurityList, error) {
	req := core.UpdateSecurityListRequest{
		SecurityListId: securityListId,
		UpdateSecurityListDetails: core.UpdateSecurityListDetails{
			IngressSecurityRules: ingressRules,
			EgressSecurityRules:  egressRules,
		},
	}
	resp, err := networkClient.UpdateSecurityList(ctx, req)
	return resp.SecurityList, err
}

// --- 监控功能 ---

func GetInstanceNetworkMetrics(instanceId, startTime, endTime string) (float64, float64, error) {
	namespace := "oci_computeagent"
	queryIn := fmt.Sprintf("NetworkBytesIn[1m]{resourceId = \"%s\"}.sum()", instanceId)
	respIn, err := queryMetrics(namespace, queryIn, startTime, endTime)
	if err != nil {
		return 0, 0, fmt.Errorf("查询流入流量失败: %v", err)
	}
	bytesIn := aggregateMetricData(respIn.Items)

	queryOut := fmt.Sprintf("NetworkBytesOut[1m]{resourceId = \"%s\"}.sum()", instanceId)
	respOut, err := queryMetrics(namespace, queryOut, startTime, endTime)
	if err != nil {
		return 0, 0, fmt.Errorf("查询流出流量失败: %v", err)
	}
	bytesOut := aggregateMetricData(respOut.Items)

	return bytesIn, bytesOut, nil
}

func queryMetrics(namespace, query, startTime, endTime string) (monitoring.SummarizeMetricsDataResponse, error) {
	req := monitoring.SummarizeMetricsDataRequest{
		CompartmentId: &oracle.Tenancy,
		SummarizeMetricsDataDetails: monitoring.SummarizeMetricsDataDetails{
			Namespace: &namespace,
			Query:     &query,
			StartTime: &common.SDKTime{Time: parseTime(startTime)},
			EndTime:   &common.SDKTime{Time: parseTime(endTime)},
		},
	}
	return monitoringClient.SummarizeMetricsData(ctx, req)
}

func aggregateMetricData(items []monitoring.MetricData) float64 {
	var total float64
	for _, item := range items {
		for _, dp := range item.AggregatedDatapoints {
			if dp.Value != nil {
				total += *dp.Value
			}
		}
	}
	return total
}

func parseTime(timeStr string) time.Time {
	t, _ := time.Parse(time.RFC3339, timeStr)
	return t
}

// --- 实例和计算功能 ---

func LaunchInstances(ads []identity.AvailabilityDomain) (sum, num int32) {
	var adCount int32 = int32(len(ads))
	adName := common.String(instance.AvailabilityDomain)
	each := instance.Each
	sum = instance.Sum
	var usableAds = make([]identity.AvailabilityDomain, 0)
	var AD_NOT_FIXED bool = false
	var EACH_AD bool = false
	if adName == nil || *adName == "" {
		AD_NOT_FIXED = true
		if each > 0 {
			EACH_AD = true
			sum = each * adCount
		} else {
			EACH_AD = false
			usableAds = ads
		}
	}
	name := instance.InstanceDisplayName
	if name == "" {
		name = time.Now().Format("instance-20060102-1504")
	}
	displayName := common.String(name)
	if sum > 1 {
		displayName = common.String(name + "-1")
	}
	request := core.LaunchInstanceRequest{}
	request.CompartmentId = common.String(oracle.Tenancy)
	request.DisplayName = displayName
	fmt.Println("正在获取系统镜像...")
	image, err := GetImage(ctx, computeClient)
	if err != nil {
		printlnErr("获取系统镜像失败", err.Error())
		return
	}
	fmt.Println("系统镜像:", *image.DisplayName)
	var shape core.Shape
	if strings.Contains(strings.ToLower(instance.Shape), "flex") && instance.Ocpus > 0 && instance.MemoryInGBs > 0 {
		shape.Shape = &instance.Shape
		shape.Ocpus = &instance.Ocpus
		shape.MemoryInGBs = &instance.MemoryInGBs
	} else {
		fmt.Println("正在获取Shape信息...")
		shape, err = getShape(image.Id, instance.Shape)
		if err != nil {
			printlnErr("获取Shape信息失败", err.Error())
			return
		}
	}
	request.Shape = shape.Shape
	if strings.Contains(strings.ToLower(*shape.Shape), "flex") {
		request.ShapeConfig = &core.LaunchInstanceShapeConfigDetails{
			Ocpus:       shape.Ocpus,
			MemoryInGBs: shape.MemoryInGBs,
		}
		if instance.Burstable == "1/8" {
			request.ShapeConfig.BaselineOcpuUtilization = core.LaunchInstanceShapeConfigDetailsBaselineOcpuUtilization8
		} else if instance.Burstable == "1/2" {
			request.ShapeConfig.BaselineOcpuUtilization = core.LaunchInstanceShapeConfigDetailsBaselineOcpuUtilization2
		}
	}
	fmt.Println("正在获取子网...")
	subnet, err := CreateOrGetNetworkInfrastructure(ctx, networkClient)
	if err != nil {
		printlnErr("获取子网失败", err.Error())
		return
	}
	fmt.Println("子网:", *subnet.DisplayName)
	request.CreateVnicDetails = &core.CreateVnicDetails{SubnetId: subnet.Id}
	sd := core.InstanceSourceViaImageDetails{}
	sd.ImageId = image.Id
	if instance.BootVolumeSizeInGBs > 0 {
		sd.BootVolumeSizeInGBs = common.Int64(instance.BootVolumeSizeInGBs)
	}
	request.SourceDetails = sd
	request.IsPvEncryptionInTransitEnabled = common.Bool(true)
	metaData := map[string]string{}
	metaData["ssh_authorized_keys"] = instance.SSH_Public_Key
	if instance.CloudInit != "" {
		metaData["user_data"] = instance.CloudInit
	}
	request.Metadata = metaData
	minTime := instance.MinTime
	maxTime := instance.MaxTime
	SKIP_RETRY_MAP := make(map[int32]bool)
	var usableAdsTemp = make([]identity.AvailabilityDomain, 0)
	retry := instance.Retry
	var failTimes int32 = 0
	var runTimes int32 = 0
	var adIndex int32 = 0
	var pos int32 = 0
	var SUCCESS = false
	var startTime = time.Now()
	var bootVolumeSize float64
	if instance.BootVolumeSizeInGBs > 0 {
		bootVolumeSize = float64(instance.BootVolumeSizeInGBs)
	} else {
		bootVolumeSize = math.Round(float64(*image.SizeInMBs) / float64(1024))
	}
	printf("\033[1;36m[%s] 开始创建 %s 实例, OCPU: %g 内存: %g 引导卷: %g \033[0m\n", oracleSectionName, *shape.Shape, *shape.Ocpus, *shape.MemoryInGBs, bootVolumeSize)
	if EACH {
		text := fmt.Sprintf("正在尝试创建第 %d 个实例...⏳\n区域: %s\n实例配置: %s\nOCPU计数: %g\n内存(GB): %g\n引导卷(GB): %g\n创建个数: %d", pos+1, oracle.Region, *shape.Shape, *shape.Ocpus, *shape.MemoryInGBs, bootVolumeSize, sum)
		_, err := sendMessage("", text)
		if err != nil {
			printlnErr("Telegram 消息提醒发送失败", err.Error())
		}
	}
	for pos < sum {
		if AD_NOT_FIXED {
			if EACH_AD {
				if pos%each == 0 && failTimes == 0 {
					adName = ads[adIndex].Name
					adIndex++
				}
			} else {
				if SUCCESS {
					adIndex = 0
				}
				if adIndex >= adCount {
					adIndex = 0
				}
				adName = usableAds[adIndex].Name
				adIndex++
			}
		}
		runTimes++
		printf("\033[1;36m[%s] 正在尝试创建第 %d 个实例, AD: %s\033[0m\n", oracleSectionName, pos+1, *adName)
		printf("\033[1;36m[%s] 当前尝试次数: %d \033[0m\n", oracleSectionName, runTimes)
		request.AvailabilityDomain = adName
		createResp, err := computeClient.LaunchInstance(ctx, request)
		if err == nil {
			SUCCESS = true
			num++
			duration := fmtDuration(time.Since(startTime))
			printf("\033[1;32m[%s] 第 %d 个实例抢到了🎉, 正在启动中请稍等...⌛️ \033[0m\n", oracleSectionName, pos+1)
			var msg Message
			var msgErr error
			var text string
			if EACH {
				text = fmt.Sprintf("第 %d 个实例抢到了🎉, 正在启动中请稍等...⌛️\n区域: %s\n实例名称: %s\n公共IP: 获取中...⏳\n可用性域:%s\n实例配置: %s\nOCPU计数: %g\n内存(GB): %g\n引导卷(GB): %g\n创建个数: %d\n尝试次数: %d\n耗时: %s", pos+1, oracle.Region, *createResp.Instance.DisplayName, *createResp.Instance.AvailabilityDomain, *shape.Shape, *shape.Ocpus, *shape.MemoryInGBs, bootVolumeSize, sum, runTimes, duration)
				msg, msgErr = sendMessage("", text)
			}
			var strIps string
			ips, err := getInstancePublicIps(createResp.Instance.Id)
			if err != nil {
				printf("\033[1;32m[%s] 第 %d 个实例抢到了🎉, 但是启动失败❌ 错误信息: \033[0m%s\n", oracleSectionName, pos+1, err.Error())
				text = fmt.Sprintf("第 %d 个实例抢到了🎉, 但是启动失败❌实例已被终止😔\n区域: %s\n实例名称: %s\n可用性域:%s\n实例配置: %s\nOCPU计数: %g\n内存(GB): %g\n引导卷(GB): %g\n创建个数: %d\n尝试次数: %d\n耗时: %s", pos+1, oracle.Region, *createResp.Instance.DisplayName, *createResp.Instance.AvailabilityDomain, *shape.Shape, *shape.Ocpus, *shape.MemoryInGBs, bootVolumeSize, sum, runTimes, duration)
			} else {
				strIps = strings.Join(ips, ",")
				printf("\033[1;32m[%s] 第 %d 个实例抢到了🎉, 启动成功✅. 实例名称: %s, 公共IP: %s\033[0m\n", oracleSectionName, pos+1, *createResp.Instance.DisplayName, strIps)
				text = fmt.Sprintf("第 %d 个实例抢到了🎉, 启动成功✅\n区域: %s\n实例名称: %s\n公共IP: %s\n可用性域:%s\n实例配置: %s\nOCPU计数: %g\n内存(GB): %g\n引导卷(GB): %g\n创建个数: %d\n尝试次数: %d\n耗时: %s", pos+1, oracle.Region, *createResp.Instance.DisplayName, strIps, *createResp.Instance.AvailabilityDomain, *shape.Shape, *shape.Ocpus, *shape.MemoryInGBs, bootVolumeSize, sum, runTimes, duration)
			}
			if EACH {
				if msgErr != nil {
					sendMessage("", text)
				} else {
					editMessage(msg.MessageId, "", text)
				}
			}
			sleepRandomSecond(minTime, maxTime)
			displayName = common.String(fmt.Sprintf("%s-%d", name, pos+1))
			request.DisplayName = displayName
		} else {
			SUCCESS = false
			errInfo := err.Error()
			SKIP_RETRY := false
			servErr, isServErr := common.IsServiceError(err)
			if isServErr && (400 <= servErr.GetHTTPStatusCode() && servErr.GetHTTPStatusCode() <= 405) || (servErr.GetHTTPStatusCode() == 409 && !strings.EqualFold(servErr.GetCode(), "IncorrectState")) || servErr.GetHTTPStatusCode() == 412 || servErr.GetHTTPStatusCode() == 413 || servErr.GetHTTPStatusCode() == 422 || servErr.GetHTTPStatusCode() == 431 || servErr.GetHTTPStatusCode() == 501 {
				if isServErr {
					errInfo = servErr.GetMessage()
				}
				duration := fmtDuration(time.Since(startTime))
				printf("\033[1;31m[%s] 第 %d 个实例创建失败了❌, 错误信息: \033[0m%s\n", oracleSectionName, pos+1, errInfo)
				if EACH {
					text := fmt.Sprintf("第 %d 个实例创建失败了❌\n错误信息: %s\n区域: %s\n可用性域: %s\n实例配置: %s\nOCPU计数: %g\n内存(GB): %g\n引导卷(GB): %g\n创建个数: %d\n尝试次数: %d\n耗时:%s", pos+1, errInfo, oracle.Region, *adName, *shape.Shape, *shape.Ocpus, *shape.MemoryInGBs, bootVolumeSize, sum, runTimes, duration)
					sendMessage("", text)
				}
				SKIP_RETRY = true
				if AD_NOT_FIXED && !EACH_AD {
					SKIP_RETRY_MAP[adIndex-1] = true
				}
			} else {
				if isServErr {
					errInfo = servErr.GetMessage()
				}
				printf("\033[1;31m[%s] 创建失败, Error: \033[0m%s\n", oracleSectionName, errInfo)
				SKIP_RETRY = false
				if AD_NOT_FIXED && !EACH_AD {
					SKIP_RETRY_MAP[adIndex-1] = false
				}
			}
			sleepRandomSecond(minTime, maxTime)
			if AD_NOT_FIXED {
				if !EACH_AD {
					if adIndex < adCount {
						continue
					} else {
						failTimes++
						for index, skip := range SKIP_RETRY_MAP {
							if !skip {
								usableAdsTemp = append(usableAdsTemp, usableAds[index])
							}
						}
						usableAds = usableAdsTemp
						adCount = int32(len(usableAds))
						usableAdsTemp = nil
						for k := range SKIP_RETRY_MAP {
							delete(SKIP_RETRY_MAP, k)
						}
						if (retry < 0 || failTimes <= retry) && adCount > 0 {
							continue
						}
					}
					adIndex = 0
				} else {
					failTimes++
					if (retry < 0 || failTimes <= retry) && !SKIP_RETRY {
						continue
					}
				}
			} else {
				failTimes++
				if (retry < 0 || failTimes <= retry) && !SKIP_RETRY {
					continue
				}
			}
		}
		usableAds = ads
		adCount = int32(len(usableAds))
		usableAdsTemp = nil
		for k := range SKIP_RETRY_MAP {
			delete(SKIP_RETRY_MAP, k)
		}
		failTimes = 0
		runTimes = 0
		startTime = time.Now()
		pos++
		if pos < sum && EACH {
			text := fmt.Sprintf("正在尝试创建第 %d 个实例...⏳\n区域: %s\n实例配置: %s\nOCPU计数: %g\n内存(GB): %g\n引导卷(GB): %g\n创建个数: %d", pos+1, oracle.Region, *shape.Shape, *shape.Ocpus, *shape.MemoryInGBs, bootVolumeSize, sum)
			sendMessage("", text)
		}
	}
	return
}

func CreateOrGetNetworkInfrastructure(ctx context.Context, c core.VirtualNetworkClient) (subnet core.Subnet, err error) {
	var vcn core.Vcn
	vcn, err = createOrGetVcn(ctx, c)
	if err != nil {
		return
	}
	var gateway core.InternetGateway
	gateway, err = createOrGetInternetGateway(c, vcn.Id)
	if err != nil {
		return
	}
	_, err = createOrGetRouteTable(c, gateway.Id, vcn.Id)
	if err != nil {
		return
	}
	subnet, err = createOrGetSubnetWithDetails(
		ctx, c, vcn.Id,
		common.String(instance.SubnetDisplayName),
		common.String("10.0.0.0/20"),
		common.String("subnetdns"),
		common.String(instance.AvailabilityDomain))
	return
}

func createOrGetSubnetWithDetails(ctx context.Context, c core.VirtualNetworkClient, vcnID *string,
	displayName *string, cidrBlock *string, dnsLabel *string, availableDomain *string) (subnet core.Subnet, err error) {
	var subnets []core.Subnet
	subnets, err = listSubnets(ctx, c, vcnID)
	if err != nil {
		return
	}
	if displayName == nil {
		displayName = common.String(instance.SubnetDisplayName)
	}
	if len(subnets) > 0 && *displayName == "" {
		subnet = subnets[0]
		return
	}
	for _, element := range subnets {
		if *element.DisplayName == *displayName {
			subnet = element
			return
		}
	}
	fmt.Printf("开始创建Subnet（没有可用的Subnet，或指定的Subnet不存在）\n")
	if *displayName == "" {
		displayName = common.String(time.Now().Format("subnet-20060102-1504"))
	}
	request := core.CreateSubnetRequest{}
	request.CompartmentId = &oracle.Tenancy
	request.CidrBlock = cidrBlock
	request.DisplayName = displayName
	request.DnsLabel = dnsLabel
	request.RequestMetadata = getCustomRequestMetadataWithRetryPolicy()
	request.VcnId = vcnID
	var r core.CreateSubnetResponse
	r, err = c.CreateSubnet(ctx, request)
	if err != nil {
		return
	}
	pollUntilAvailable := func(r common.OCIOperationResponse) bool {
		if converted, ok := r.Response.(core.GetSubnetResponse); ok {
			return converted.LifecycleState != core.SubnetLifecycleStateAvailable
		}
		return true
	}
	pollGetRequest := core.GetSubnetRequest{
		SubnetId:        r.Id,
		RequestMetadata: getCustomRequestMetadataWithCustomizedRetryPolicy(pollUntilAvailable),
	}
	_, err = c.GetSubnet(ctx, pollGetRequest)
	if err != nil {
		return
	}
	getReq := core.GetSecurityListRequest{
		SecurityListId:  common.String(r.SecurityListIds[0]),
		RequestMetadata: getCustomRequestMetadataWithRetryPolicy(),
	}
	var getResp core.GetSecurityListResponse
	getResp, err = c.GetSecurityList(ctx, getReq)
	if err != nil {
		return
	}
	newRules := append(getResp.IngressSecurityRules, core.IngressSecurityRule{
		Protocol: common.String("all"),
		Source:   common.String("0.0.0.0/0"),
	})
	updateReq := core.UpdateSecurityListRequest{
		SecurityListId:  common.String(r.SecurityListIds[0]),
		RequestMetadata: getCustomRequestMetadataWithRetryPolicy(),
	}
	updateReq.IngressSecurityRules = newRules
	_, err = c.UpdateSecurityList(ctx, updateReq)
	if err != nil {
		return
	}
	fmt.Printf("Subnet创建成功: %s\n", *r.Subnet.DisplayName)
	subnet = r.Subnet
	return
}

func listSubnets(ctx context.Context, c core.VirtualNetworkClient, vcnID *string) (subnets []core.Subnet, err error) {
	request := core.ListSubnetsRequest{
		CompartmentId:   &oracle.Tenancy,
		VcnId:           vcnID,
		RequestMetadata: getCustomRequestMetadataWithRetryPolicy(),
	}
	var r core.ListSubnetsResponse
	r, err = c.ListSubnets(ctx, request)
	if err != nil {
		return
	}
	subnets = r.Items
	return
}

func createOrGetVcn(ctx context.Context, c core.VirtualNetworkClient) (core.Vcn, error) {
	var vcn core.Vcn
	vcnItems, err := listVcns(ctx, c)
	if err != nil {
		return vcn, err
	}
	displayName := common.String(instance.VcnDisplayName)
	if len(vcnItems) > 0 && *displayName == "" {
		vcn = vcnItems[0]
		return vcn, err
	}
	for _, element := range vcnItems {
		if *element.DisplayName == instance.VcnDisplayName {
			vcn = element
			return vcn, err
		}
	}
	fmt.Println("开始创建VCN（没有可用的VCN，或指定的VCN不存在）")
	if *displayName == "" {
		displayName = common.String(time.Now().Format("vcn-20060102-1504"))
	}
	request := core.CreateVcnRequest{}
	request.RequestMetadata = getCustomRequestMetadataWithRetryPolicy()
	request.CidrBlock = common.String("10.0.0.0/16")
	request.CompartmentId = common.String(oracle.Tenancy)
	request.DisplayName = displayName
	request.DnsLabel = common.String("vcndns")
	r, err := c.CreateVcn(ctx, request)
	if err != nil {
		return vcn, err
	}
	fmt.Printf("VCN创建成功: %s\n", *r.Vcn.DisplayName)
	vcn = r.Vcn
	return vcn, err
}

func listVcns(ctx context.Context, c core.VirtualNetworkClient) ([]core.Vcn, error) {
	request := core.ListVcnsRequest{
		CompartmentId:   &oracle.Tenancy,
		RequestMetadata: getCustomRequestMetadataWithRetryPolicy(),
	}
	r, err := c.ListVcns(ctx, request)
	if err != nil {
		return nil, err
	}
	return r.Items, err
}

func createOrGetInternetGateway(c core.VirtualNetworkClient, vcnID *string) (core.InternetGateway, error) {
	var gateway core.InternetGateway
	listGWRequest := core.ListInternetGatewaysRequest{
		CompartmentId:   &oracle.Tenancy,
		VcnId:           vcnID,
		RequestMetadata: getCustomRequestMetadataWithRetryPolicy(),
	}
	listGWRespone, err := c.ListInternetGateways(ctx, listGWRequest)
	if err != nil {
		return gateway, err
	}
	if len(listGWRespone.Items) >= 1 {
		gateway = listGWRespone.Items[0]
	} else {
		fmt.Printf("开始创建Internet网关\n")
		enabled := true
		createGWDetails := core.CreateInternetGatewayDetails{
			CompartmentId: &oracle.Tenancy,
			IsEnabled:     &enabled,
			VcnId:         vcnID,
		}
		createGWRequest := core.CreateInternetGatewayRequest{
			CreateInternetGatewayDetails: createGWDetails,
			RequestMetadata:              getCustomRequestMetadataWithRetryPolicy()}
		createGWResponse, err := c.CreateInternetGateway(ctx, createGWRequest)
		if err != nil {
			return gateway, err
		}
		gateway = createGWResponse.InternetGateway
		fmt.Printf("Internet网关创建成功: %s\n", *gateway.DisplayName)
	}
	return gateway, err
}

func createOrGetRouteTable(c core.VirtualNetworkClient, gatewayID, VcnID *string) (routeTable core.RouteTable, err error) {
	listRTRequest := core.ListRouteTablesRequest{
		CompartmentId:   &oracle.Tenancy,
		VcnId:           VcnID,
		RequestMetadata: getCustomRequestMetadataWithRetryPolicy(),
	}
	var listRTResponse core.ListRouteTablesResponse
	listRTResponse, err = c.ListRouteTables(ctx, listRTRequest)
	if err != nil {
		return
	}
	cidrRange := "0.0.0.0/0"
	rr := core.RouteRule{
		NetworkEntityId: gatewayID,
		Destination:     &cidrRange,
		DestinationType: core.RouteRuleDestinationTypeCidrBlock,
	}
	if len(listRTResponse.Items) >= 1 {
		if len(listRTResponse.Items[0].RouteRules) >= 1 {
			routeTable = listRTResponse.Items[0]
		} else {
			fmt.Printf("路由表未添加规则，开始添加Internet路由规则\n")
			updateRTDetails := core.UpdateRouteTableDetails{
				RouteRules: []core.RouteRule{rr},
			}
			updateRTRequest := core.UpdateRouteTableRequest{
				RtId:                    listRTResponse.Items[0].Id,
				UpdateRouteTableDetails: updateRTDetails,
				RequestMetadata:         getCustomRequestMetadataWithRetryPolicy(),
			}
			var updateRTResponse core.UpdateRouteTableResponse
			updateRTResponse, err = c.UpdateRouteTable(ctx, updateRTRequest)
			if err != nil {
				return
			}
			fmt.Printf("Internet路由规则添加成功\n")
			routeTable = updateRTResponse.RouteTable
		}
	} else {
		fmt.Printf("错误: 找不到VCN的默认路由表, VCN OCID: %s\n", *VcnID)
	}
	return
}

func GetImage(ctx context.Context, c core.ComputeClient) (image core.Image, err error) {
	var images []core.Image
	images, err = listImages(ctx, c)
	if err != nil {
		return
	}
	if len(images) > 0 {
		image = images[0]
	} else {
		err = fmt.Errorf("未找到[%s %s]的镜像, 或该镜像不支持[%s]", instance.OperatingSystem, instance.OperatingSystemVersion, instance.Shape)
	}
	return
}

func listImages(ctx context.Context, c core.ComputeClient) ([]core.Image, error) {
	if instance.OperatingSystem == "" || instance.OperatingSystemVersion == "" {
		return nil, errors.New("操作系统类型和版本不能为空, 请检查配置文件")
	}
	request := core.ListImagesRequest{
		CompartmentId:          common.String(oracle.Tenancy),
		OperatingSystem:        common.String(instance.OperatingSystem),
		OperatingSystemVersion: common.String(instance.OperatingSystemVersion),
		Shape:                  common.String(instance.Shape),
		RequestMetadata:        getCustomRequestMetadataWithRetryPolicy(),
	}
	r, err := c.ListImages(ctx, request)
	return r.Items, err
}

func getShape(imageId *string, shapeName string) (core.Shape, error) {
	var shape core.Shape
	shapes, err := listShapes(ctx, computeClient, imageId)
	if err != nil {
		return shape, err
	}
	for _, s := range shapes {
		if strings.EqualFold(*s.Shape, shapeName) {
			shape = s
			return shape, nil
		}
	}
	return shape, errors.New("没有符合条件的Shape")
}

func listShapes(ctx context.Context, c core.ComputeClient, imageID *string) ([]core.Shape, error) {
	request := core.ListShapesRequest{
		CompartmentId:   common.String(oracle.Tenancy),
		ImageId:         imageID,
		RequestMetadata: getCustomRequestMetadataWithRetryPolicy(),
	}
	r, err := c.ListShapes(ctx, request)
	if err == nil && (r.Items == nil || len(r.Items) == 0) {
		err = errors.New("没有符合条件的Shape")
	}
	return r.Items, err
}

func ListAvailabilityDomains() ([]identity.AvailabilityDomain, error) {
	req := identity.ListAvailabilityDomainsRequest{
		CompartmentId:   common.String(oracle.Tenancy),
		RequestMetadata: getCustomRequestMetadataWithRetryPolicy(),
	}
	resp, err := identityClient.ListAvailabilityDomains(ctx, req)
	return resp.Items, err
}

func ListInstances(ctx context.Context, c core.ComputeClient, page *string) ([]core.Instance, *string, error) {
	req := core.ListInstancesRequest{
		CompartmentId:   common.String(oracle.Tenancy),
		RequestMetadata: getCustomRequestMetadataWithRetryPolicy(),
		Limit:           common.Int(100),
		Page:            page,
	}
	resp, err := c.ListInstances(ctx, req)
	return resp.Items, resp.OpcNextPage, err
}

func ListVnicAttachments(ctx context.Context, c core.ComputeClient, instanceId *string, page *string) ([]core.VnicAttachment, *string, error) {
	req := core.ListVnicAttachmentsRequest{
		CompartmentId:   common.String(oracle.Tenancy),
		RequestMetadata: getCustomRequestMetadataWithRetryPolicy(),
		Limit:           common.Int(100),
		Page:            page,
	}
	if instanceId != nil && *instanceId != "" {
		req.InstanceId = instanceId
	}
	resp, err := c.ListVnicAttachments(ctx, req)
	return resp.Items, resp.OpcNextPage, err
}

func GetVnic(ctx context.Context, c core.VirtualNetworkClient, vnicID *string) (core.Vnic, error) {
	req := core.GetVnicRequest{
		VnicId:          vnicID,
		RequestMetadata: getCustomRequestMetadataWithRetryPolicy(),
	}
	resp, err := c.GetVnic(ctx, req)
	if err != nil && resp.RawResponse != nil {
		err = errors.New(resp.RawResponse.Status)
	}
	return resp.Vnic, err
}

func terminateInstance(id *string) error {
	request := core.TerminateInstanceRequest{
		InstanceId:         id,
		PreserveBootVolume: common.Bool(false),
		RequestMetadata:    getCustomRequestMetadataWithRetryPolicy(),
	}
	_, err := computeClient.TerminateInstance(ctx, request)
	return err
}

func getInstance(instanceId *string) (core.Instance, error) {
	req := core.GetInstanceRequest{
		InstanceId:      instanceId,
		RequestMetadata: getCustomRequestMetadataWithRetryPolicy(),
	}
	resp, err := computeClient.GetInstance(ctx, req)
	return resp.Instance, err
}

func updateInstance(instanceId *string, displayName *string, ocpus, memoryInGBs *float32,
	details []core.InstanceAgentPluginConfigDetails, disable *bool) (core.UpdateInstanceResponse, error) {
	updateInstanceDetails := core.UpdateInstanceDetails{}
	if displayName != nil && *displayName != "" {
		updateInstanceDetails.DisplayName = displayName
	}
	shapeConfig := core.UpdateInstanceShapeConfigDetails{}
	if ocpus != nil && *ocpus > 0 {
		shapeConfig.Ocpus = ocpus
	}
	if memoryInGBs != nil && *memoryInGBs > 0 {
		shapeConfig.MemoryInGBs = memoryInGBs
	}
	updateInstanceDetails.ShapeConfig = &shapeConfig
	if disable != nil && details != nil {
		for i := 0; i < len(details); i++ {
			if *disable {
				details[i].DesiredState = core.InstanceAgentPluginConfigDetailsDesiredStateDisabled
			} else {
				details[i].DesiredState = core.InstanceAgentPluginConfigDetailsDesiredStateEnabled
			}
		}
		agentConfig := core.UpdateInstanceAgentConfigDetails{
			IsMonitoringDisabled:  disable,
			IsManagementDisabled:  disable,
			AreAllPluginsDisabled: disable,
			PluginsConfig:         details,
		}
		updateInstanceDetails.AgentConfig = &agentConfig
	}
	req := core.UpdateInstanceRequest{
		InstanceId:            instanceId,
		UpdateInstanceDetails: updateInstanceDetails,
		RequestMetadata:       getCustomRequestMetadataWithRetryPolicy(),
	}
	return computeClient.UpdateInstance(ctx, req)
}

func instanceAction(instanceId *string, action core.InstanceActionActionEnum) (core.Instance, error) {
	req := core.InstanceActionRequest{
		InstanceId:      instanceId,
		Action:          action,
		RequestMetadata: getCustomRequestMetadataWithRetryPolicy(),
	}
	resp, err := computeClient.InstanceAction(ctx, req)
	return resp.Instance, err
}

func changePublicIp(vnics []core.Vnic) (publicIp core.PublicIp, err error) {
	var vnic core.Vnic
	for _, v := range vnics {
		if v.IsPrimary != nil && *v.IsPrimary {
			vnic = v
		}
	}
	fmt.Println("正在获取私有IP...")
	var privateIps []core.PrivateIp
	privateIps, err = getPrivateIps(vnic.Id)
	if err != nil {
		printlnErr("获取私有IP失败", err.Error())
		return
	}
	var privateIp core.PrivateIp
	for _, p := range privateIps {
		if p.IsPrimary != nil && *p.IsPrimary {
			privateIp = p
		}
	}
	fmt.Println("正在获取公共IP OCID...")
	publicIp, err = getPublicIp(privateIp.Id)
	if err != nil {
		printlnErr("获取公共IP OCID 失败", err.Error())
	}
	fmt.Println("正在删除公共IP...")
	_, err = deletePublicIp(publicIp.Id)
	if err != nil {
		printlnErr("删除公共IP 失败", err.Error())
	}
	time.Sleep(3 * time.Second)
	fmt.Println("正在创建公共IP...")
	publicIp, err = createPublicIp(privateIp.Id)
	return
}

func getInstanceVnics(instanceId *string) (vnics []core.Vnic, err error) {
	vnicAttachments, _, err := ListVnicAttachments(ctx, computeClient, instanceId, nil)
	if err != nil {
		return
	}
	for _, vnicAttachment := range vnicAttachments {
		vnic, vnicErr := GetVnic(ctx, networkClient, vnicAttachment.VnicId)
		if vnicErr != nil {
			fmt.Printf("GetVnic error: %s\n", vnicErr.Error())
			continue
		}
		vnics = append(vnics, vnic)
	}
	return
}

func getPrivateIps(vnicId *string) ([]core.PrivateIp, error) {
	req := core.ListPrivateIpsRequest{
		VnicId:          vnicId,
		RequestMetadata: getCustomRequestMetadataWithRetryPolicy(),
	}
	resp, err := networkClient.ListPrivateIps(ctx, req)
	if err == nil && (resp.Items == nil || len(resp.Items) == 0) {
		err = errors.New("私有IP为空")
	}
	return resp.Items, err
}

func getPublicIp(privateIpId *string) (core.PublicIp, error) {
	req := core.GetPublicIpByPrivateIpIdRequest{
		GetPublicIpByPrivateIpIdDetails: core.GetPublicIpByPrivateIpIdDetails{PrivateIpId: privateIpId},
		RequestMetadata:                 getCustomRequestMetadataWithRetryPolicy(),
	}
	resp, err := networkClient.GetPublicIpByPrivateIpId(ctx, req)
	if err == nil && resp.PublicIp.Id == nil {
		err = errors.New("未分配公共IP")
	}
	return resp.PublicIp, err
}

func deletePublicIp(publicIpId *string) (core.DeletePublicIpResponse, error) {
	req := core.DeletePublicIpRequest{
		PublicIpId:      publicIpId,
		RequestMetadata: getCustomRequestMetadataWithRetryPolicy()}
	return networkClient.DeletePublicIp(ctx, req)
}

func createPublicIp(privateIpId *string) (core.PublicIp, error) {
	req := core.CreatePublicIpRequest{
		CreatePublicIpDetails: core.CreatePublicIpDetails{
			CompartmentId: common.String(oracle.Tenancy),
			Lifetime:      core.CreatePublicIpDetailsLifetimeEphemeral,
			PrivateIpId:   privateIpId,
		},
		RequestMetadata: getCustomRequestMetadataWithRetryPolicy(),
	}
	resp, err := networkClient.CreatePublicIp(ctx, req)
	return resp.PublicIp, err
}

func getInstancePublicIps(instanceId *string) (ips []string, err error) {
	var ins core.Instance
	for i := 0; i < 100; i++ {
		if ins.LifecycleState != core.InstanceLifecycleStateRunning {
			ins, err = getInstance(instanceId)
			if err != nil {
				continue
			}
			if ins.LifecycleState == core.InstanceLifecycleStateTerminating || ins.LifecycleState == core.InstanceLifecycleStateTerminated {
				err = errors.New("实例已终止😔")
				return
			}
		}
		var vnicAttachments []core.VnicAttachment
		vnicAttachments, _, err = ListVnicAttachments(ctx, computeClient, instanceId, nil)
		if err != nil {
			continue
		}
		if len(vnicAttachments) > 0 {
			for _, vnicAttachment := range vnicAttachments {
				vnic, vnicErr := GetVnic(ctx, networkClient, vnicAttachment.VnicId)
				if vnicErr != nil {
					printf("GetVnic error: %s\n", vnicErr.Error())
					continue
				}
				if vnic.PublicIp != nil && *vnic.PublicIp != "" {
					ips = append(ips, *vnic.PublicIp)
				}
			}
			return
		}
		time.Sleep(3 * time.Second)
	}
	return
}

func getBootVolumes(availabilityDomain *string) ([]core.BootVolume, error) {
	req := core.ListBootVolumesRequest{
		AvailabilityDomain: availabilityDomain,
		CompartmentId:      common.String(oracle.Tenancy),
		RequestMetadata:    getCustomRequestMetadataWithRetryPolicy(),
	}
	resp, err := storageClient.ListBootVolumes(ctx, req)
	return resp.Items, err
}

func getBootVolume(bootVolumeId *string) (core.BootVolume, error) {
	req := core.GetBootVolumeRequest{
		BootVolumeId:    bootVolumeId,
		RequestMetadata: getCustomRequestMetadataWithRetryPolicy(),
	}
	resp, err := storageClient.GetBootVolume(ctx, req)
	return resp.BootVolume, err
}

func updateBootVolume(bootVolumeId *string, sizeInGBs *int64, vpusPerGB *int64) (core.BootVolume, error) {
	updateBootVolumeDetails := core.UpdateBootVolumeDetails{}
	if sizeInGBs != nil {
		updateBootVolumeDetails.SizeInGBs = sizeInGBs
	}
	if vpusPerGB != nil {
		updateBootVolumeDetails.VpusPerGB = vpusPerGB
	}
	req := core.UpdateBootVolumeRequest{
		BootVolumeId:            bootVolumeId,
		UpdateBootVolumeDetails: updateBootVolumeDetails,
		RequestMetadata:         getCustomRequestMetadataWithRetryPolicy(),
	}
	resp, err := storageClient.UpdateBootVolume(ctx, req)
	return resp.BootVolume, err
}

func deleteBootVolume(bootVolumeId *string) (*http.Response, error) {
	req := core.DeleteBootVolumeRequest{
		BootVolumeId:    bootVolumeId,
		RequestMetadata: getCustomRequestMetadataWithRetryPolicy(),
	}
	resp, err := storageClient.DeleteBootVolume(ctx, req)
	return resp.RawResponse, err
}

func detachBootVolume(bootVolumeAttachmentId *string) (*http.Response, error) {
	req := core.DetachBootVolumeRequest{
		BootVolumeAttachmentId: bootVolumeAttachmentId,
		RequestMetadata:        getCustomRequestMetadataWithRetryPolicy(),
	}
	resp, err := computeClient.DetachBootVolume(ctx, req)
	return resp.RawResponse, err
}

func listBootVolumeAttachments(availabilityDomain, compartmentId, bootVolumeId *string) ([]core.BootVolumeAttachment, error) {
	req := core.ListBootVolumeAttachmentsRequest{
		AvailabilityDomain: availabilityDomain,
		CompartmentId:      compartmentId,
		BootVolumeId:       bootVolumeId,
		RequestMetadata:    getCustomRequestMetadataWithRetryPolicy(),
	}
	resp, err := computeClient.ListBootVolumeAttachments(ctx, req)
	return resp.Items, err
}
