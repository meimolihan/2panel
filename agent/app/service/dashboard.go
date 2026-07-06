package service

import (
	"cmp"
	"encoding/json"
	"fmt"
	network "net"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/2Panel-dev/2Panel/agent/app/dto"
	"github.com/2Panel-dev/2Panel/agent/constant"
	"github.com/2Panel-dev/2Panel/agent/global"
	"github.com/2Panel-dev/2Panel/agent/utils/cmd"
	"github.com/2Panel-dev/2Panel/agent/utils/controller"
	"github.com/2Panel-dev/2Panel/agent/utils/psutil"
	"github.com/gin-gonic/gin"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
)

type DashboardService struct{}

type IDashboardService interface {
	LoadOsInfo() (*dto.OsInfo, error)
	LoadBaseInfo(ioOption string, netOption string) (*dto.DashboardBase, error)
	LoadCurrentInfoForNode() *dto.NodeCurrent
	LoadCurrentInfo(ioOption string, netOption string) *dto.DashboardCurrent
	LoadTopCPU() []dto.Process
	LoadTopMem() []dto.Process

	LoadQuickOptions() []dto.QuickJump
	ChangeQuick(req dto.ChangeQuicks) error

	LoadAppLauncher(ctx *gin.Context) ([]dto.AppLauncher, error)
	ChangeShow(req dto.SettingUpdate) error
	ListLauncherOption(filter string) ([]dto.LauncherOption, error)
	Restart(operation string) error
}

func NewIDashboardService() IDashboardService {
	return &DashboardService{}
}

func (u *DashboardService) Restart(operation string) error {
	switch operation {
	case "system":
		{
			go func() {
				cmdMgr := cmd.NewCommandMgr()
				err := cmdMgr.RunWithOptionalSudo("reboot")
				if err != nil {
					global.LOG.Errorf("handle reboot failed, %v", err)
				}
			}()
			return nil
		}
	case "2panel-agent":
		controller.RestartPanel(false, true, false)
		return nil
	case "2panel":
		controller.RestartPanel(true, true, false)
		return nil
	default:
		return fmt.Errorf("handle restart operation %s failed, err: nonsupport such operation", operation)
	}
}

func (u *DashboardService) LoadOsInfo() (*dto.OsInfo, error) {
	var baseInfo dto.OsInfo
	hostInfo, err := psutil.HOST.GetHostInfo(false)
	if err != nil {
		return nil, err
	}
	baseInfo.OS = hostInfo.OS
	baseInfo.Platform = hostInfo.Platform
	baseInfo.PlatformFamily = hostInfo.PlatformFamily
	baseInfo.KernelArch = hostInfo.KernelArch
	baseInfo.KernelVersion = hostInfo.KernelVersion
	baseInfo.PrettyDistro = psutil.HOST.GetDistro()

	diskInfo, err := psutil.DISK.GetUsage(global.Dir.BaseDir, false)
	if err == nil {
		baseInfo.DiskSize = int64(diskInfo.Free)
	}

	if baseInfo.KernelArch == "armv7l" {
		baseInfo.KernelArch = "armv7"
	}
	if baseInfo.KernelArch == "x86_64" {
		baseInfo.KernelArch = "amd64"
	}
	return &baseInfo, nil
}

func (u *DashboardService) LoadCurrentInfoForNode() *dto.NodeCurrent {
	var currentInfo dto.NodeCurrent

	currentInfo.CPUTotal, _ = psutil.CPUInfo.GetLogicalCores(false)

	cpuUsedPercent, perCore, cpuDetailedPercent := psutil.CPU.GetCPUUsage()
	if len(perCore) == 0 {
		currentInfo.CPUTotal = psutil.CPU.NumCPU()
	} else {
		currentInfo.CPUTotal = len(perCore)
	}
	currentInfo.CPUUsedPercent = cpuUsedPercent
	currentInfo.CPUUsed = cpuUsedPercent * 0.01 * float64(currentInfo.CPUTotal)
	currentInfo.CPUDetailedPercent = cpuDetailedPercent

	loadInfo, _ := load.Avg()
	currentInfo.Load1 = loadInfo.Load1
	currentInfo.Load5 = loadInfo.Load5
	currentInfo.Load15 = loadInfo.Load15
	currentInfo.LoadUsagePercent = loadInfo.Load1 / (float64(currentInfo.CPUTotal*2) * 0.75) * 100

	memoryInfo, _ := mem.VirtualMemory()
	currentInfo.MemoryTotal = memoryInfo.Total
	currentInfo.MemoryAvailable = memoryInfo.Available
	currentInfo.MemoryUsed = memoryInfo.Used
	currentInfo.MemoryUsedPercent = memoryInfo.UsedPercent

	swapInfo, _ := mem.SwapMemory()
	currentInfo.SwapMemoryTotal = swapInfo.Total
	currentInfo.SwapMemoryAvailable = swapInfo.Free
	currentInfo.SwapMemoryUsed = swapInfo.Used
	currentInfo.SwapMemoryUsedPercent = swapInfo.UsedPercent

	return &currentInfo
}

func (u *DashboardService) LoadBaseInfo(ioOption string, netOption string) (*dto.DashboardBase, error) {
	var baseInfo dto.DashboardBase
	hostInfo, err := psutil.HOST.GetHostInfo(true)
	if err != nil {
		return nil, err
	}
	ss, _ := json.Marshal(hostInfo)
	baseInfo = dto.DashboardBase{
		Hostname:             hostInfo.Hostname,
		OS:                   hostInfo.OS,
		Platform:             hostInfo.Platform,
		PlatformFamily:       hostInfo.PlatformFamily,
		PlatformVersion:      hostInfo.PlatformVersion,
		PrettyDistro:         psutil.HOST.GetDistro(),
		KernelArch:           hostInfo.KernelArch,
		KernelVersion:        hostInfo.KernelVersion,
		VirtualizationSystem: string(ss),
		IpV4Addr:             loadOutboundIP(),
		SystemProxy:          "noProxy",
	}

	if proxy := cmp.Or(os.Getenv("http_proxy"), os.Getenv("HTTP_PROXY")); proxy != "" {
		baseInfo.SystemProxy = proxy
	}

	loadQuickJump(&baseInfo)

	cpuInfo, err := psutil.CPUInfo.GetCPUInfo(false)
	if err == nil && len(cpuInfo) > 0 {
		baseInfo.CPUModelName = cpuInfo[0].ModelName
	}

	baseInfo.CPUCores, _ = psutil.CPUInfo.GetPhysicalCores(false)
	baseInfo.CPULogicalCores, _ = psutil.CPUInfo.GetLogicalCores(false)
	baseInfo.CPUMhz = cpuInfo[0].Mhz

	baseInfo.CurrentInfo = *u.LoadCurrentInfo(ioOption, netOption)
	return &baseInfo, nil
}

func (u *DashboardService) LoadCurrentInfo(ioOption string, netOption string) *dto.DashboardCurrent {
	var currentInfo dto.DashboardCurrent
	hostInfo, _ := psutil.HOST.GetHostInfo(false)
	currentInfo.Uptime = hostInfo.Uptime
	currentInfo.TimeSinceUptime = time.Unix(int64(hostInfo.BootTime), 0).Format(constant.DateTimeLayout)
	currentInfo.RunningTime = loadRunningTime(hostInfo.Uptime)
	currentInfo.Procs = hostInfo.Procs
	currentInfo.CPUTotal, _ = psutil.CPUInfo.GetLogicalCores(false)

	cpuUsedPercent, perCore, cpuDetailedPercent := psutil.CPU.GetCPUUsage()
	if len(perCore) == 0 {
		currentInfo.CPUTotal = psutil.CPU.NumCPU()
	} else {
		currentInfo.CPUTotal = len(perCore)
	}
	currentInfo.CPUPercent = perCore
	currentInfo.CPUUsedPercent = cpuUsedPercent
	currentInfo.CPUUsed = cpuUsedPercent * 0.01 * float64(currentInfo.CPUTotal)
	currentInfo.CPUDetailedPercent = cpuDetailedPercent

	loadInfo, _ := load.Avg()
	currentInfo.Load1 = loadInfo.Load1
	currentInfo.Load5 = loadInfo.Load5
	currentInfo.Load15 = loadInfo.Load15
	currentInfo.LoadUsagePercent = loadInfo.Load1 / (float64(currentInfo.CPUTotal*2) * 0.75) * 100

	memoryInfo, _ := mem.VirtualMemory()
	currentInfo.MemoryTotal = memoryInfo.Total
	currentInfo.MemoryUsed = memoryInfo.Used
	currentInfo.MemoryFree = memoryInfo.Free
	currentInfo.MemoryCache = memoryInfo.Cached + memoryInfo.Buffers
	currentInfo.MemoryShard = memoryInfo.Shared
	currentInfo.MemoryAvailable = memoryInfo.Available
	currentInfo.MemoryUsedPercent = memoryInfo.UsedPercent

	swapInfo, _ := mem.SwapMemory()
	currentInfo.SwapMemoryTotal = swapInfo.Total
	currentInfo.SwapMemoryAvailable = swapInfo.Free
	currentInfo.SwapMemoryUsed = swapInfo.Used
	currentInfo.SwapMemoryUsedPercent = swapInfo.UsedPercent

	currentInfo.DiskData = loadDiskInfo()
	currentInfo.GPUData = nil
	currentInfo.XPUData = nil

	if ioOption == "all" {
		diskInfo, _ := disk.IOCounters()
		for _, state := range diskInfo {
			currentInfo.IOReadBytes += state.ReadBytes
			currentInfo.IOWriteBytes += state.WriteBytes
			currentInfo.IOCount += (state.ReadCount + state.WriteCount)
			currentInfo.IOReadTime += state.ReadTime
			currentInfo.IOWriteTime += state.WriteTime
		}
	} else {
		diskInfo, _ := disk.IOCounters(ioOption)
		for _, state := range diskInfo {
			currentInfo.IOReadBytes += state.ReadBytes
			currentInfo.IOWriteBytes += state.WriteBytes
			currentInfo.IOCount += (state.ReadCount + state.WriteCount)
			currentInfo.IOReadTime += state.ReadTime
			currentInfo.IOWriteTime += state.WriteTime
		}
	}

	if netOption == "all" {
		netInfo, _ := net.IOCounters(false)
		if len(netInfo) != 0 {
			currentInfo.NetBytesSent = netInfo[0].BytesSent
			currentInfo.NetBytesRecv = netInfo[0].BytesRecv
		}
	} else {
		netInfo, _ := net.IOCounters(true)
		for _, state := range netInfo {
			if state.Name == netOption {
				currentInfo.NetBytesSent = state.BytesSent
				currentInfo.NetBytesRecv = state.BytesRecv
				break
			}
		}
	}

	currentInfo.ShotTime = time.Now()
	return &currentInfo
}

func loadRunningTime(uptime uint64) dto.RunningTime {
	return dto.RunningTime{
		Days:    uptime / 86400,
		Hours:   (uptime % 86400) / 3600,
		Minutes: (uptime % 3600) / 60,
		Seconds: uptime % 60,
	}
}

func (u *DashboardService) LoadTopCPU() []dto.Process {
	return loadTopCPU()
}

func (u *DashboardService) LoadTopMem() []dto.Process {
	return loadTopMem()
}

func (u *DashboardService) LoadAppLauncher(ctx *gin.Context) ([]dto.AppLauncher, error) {
	return nil, nil
}

func (u *DashboardService) ChangeShow(req dto.SettingUpdate) error {
	return nil
}

func (u *DashboardService) LoadQuickOptions() []dto.QuickJump {
	return nil
}
func (u *DashboardService) ChangeQuick(req dto.ChangeQuicks) error {
	return nil
}

func (u *DashboardService) ListLauncherOption(filter string) ([]dto.LauncherOption, error) {
	return nil, nil
}

type diskInfo struct {
	Type   string
	Mount  string
	Device string
}

func loadDiskInfo() []dto.DiskInfo {
	var datas []dto.DiskInfo
	cmdMgr := cmd.NewCommandMgr(cmd.WithTimeout(2 * time.Second))
	format := `NR>1 && !/tmpfs|snap\/core|udev/ {printf "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", $1, $2, $3, $4, $5, $6, $7}`
	stdout, err := cmdMgr.RunPipe(
		cmd.PipeCommand{Name: "df", Args: []string{"-hT", "-P"}},
		cmd.PipeCommand{Name: "awk", Args: []string{format}},
	)
	if err != nil {
		cmdMgr2 := cmd.NewCommandMgr(cmd.WithTimeout(1 * time.Second))
		stdout, err = cmdMgr2.RunPipe(
			cmd.PipeCommand{Name: "df", Args: []string{"-lhT", "-P"}},
			cmd.PipeCommand{Name: "awk", Args: []string{format}},
		)
		if err != nil {
			return datas
		}
	}
	lines := strings.Split(stdout, "\n")

	var mounts []diskInfo
	var excludes = []string{"/mnt/cdrom", "/boot", "/boot/efi", "/dev", "/dev/shm", "/run/lock", "/run", "/run/shm", "/run/user"}
	for _, line := range lines {
		fields := strings.Split(line, "\t")
		if len(fields) < 7 {
			continue
		}
		if strings.HasPrefix(fields[6], "/snap") || len(strings.Split(fields[6], "/")) > 10 {
			continue
		}
		if strings.TrimSpace(fields[1]) == "tmpfs" || strings.TrimSpace(fields[1]) == "overlay" {
			continue
		}
		if strings.Contains(fields[2], "K") {
			continue
		}
		if strings.Contains(fields[6], "docker") || strings.Contains(fields[6], "podman") || strings.Contains(fields[6], "containerd") || strings.HasPrefix(fields[6], "/var/lib/containers") {
			continue
		}
		isExclude := false
		for _, exclude := range excludes {
			if exclude == fields[6] {
				isExclude = true
			}
		}
		if isExclude {
			continue
		}
		mounts = append(mounts, diskInfo{Type: fields[1], Device: fields[0], Mount: strings.Join(fields[6:], " ")})
	}

	var (
		wg sync.WaitGroup
		mu sync.Mutex
	)
	wg.Add(len(mounts))
	for i := 0; i < len(mounts); i++ {
		go func(mount diskInfo) {
			defer wg.Done()

			var itemData dto.DiskInfo
			itemData.Path = mount.Mount
			itemData.Type = mount.Type
			itemData.Device = mount.Device

			type diskResult struct {
				state *disk.UsageStat
				err   error
			}
			resultCh := make(chan diskResult, 1)

			go func() {
				state, err := psutil.DISK.GetUsage(mount.Mount, false)
				resultCh <- diskResult{state: state, err: err}
			}()

			select {
			case <-time.After(5 * time.Second):
				mu.Lock()
				datas = append(datas, itemData)
				mu.Unlock()
				global.LOG.Errorf("load disk info from %s failed, err: timeout", mount.Mount)
			case result := <-resultCh:
				if result.err != nil {
					mu.Lock()
					datas = append(datas, itemData)
					mu.Unlock()
					global.LOG.Errorf("load disk info from %s failed, err: %v", mount.Mount, result.err)
					return
				}
				itemData.Total = result.state.Total
				itemData.Free = result.state.Free
				itemData.Used = result.state.Used
				itemData.UsedPercent = result.state.UsedPercent
				itemData.InodesTotal = result.state.InodesTotal
				itemData.InodesUsed = result.state.InodesUsed
				itemData.InodesFree = result.state.InodesFree
				itemData.InodesUsedPercent = result.state.InodesUsedPercent
				mu.Lock()
				datas = append(datas, itemData)
				mu.Unlock()
			}
		}(mounts[i])
	}
	wg.Wait()

	sort.Slice(datas, func(i, j int) bool {
		return datas[i].Path < datas[j].Path
	})
	return datas
}

type AppLauncher struct {
	Key string `json:"key"`
}

func ArryContains(arr []string, element string) bool {
	for _, v := range arr {
		if v == element {
			return true
		}
	}
	return false
}

func loadOutboundIP() string {
	conn, err := network.Dial("udp", "8.8.8.8:80")

	if err != nil {
		return "IPNotFound"
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*network.UDPAddr)
	return localAddr.IP.String()
}

func loadQuickJump(base *dto.DashboardBase) {
	cronjobs, _ := cronjobRepo.List()
	base.CronjobNumber = len(cronjobs)
}
