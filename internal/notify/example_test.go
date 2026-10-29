package notify_test

import (
	"github.com/2panel-dev/2panel/internal/notify"
)

// 使用前通过环境变量配置（示例）：
//
//	BARK_ENABLE=true
//	BARK_KEY=your-device-key
//	BARK_SERVER=https://api.day.app
func Example() {
	// 分级通知：成功 / 失败 / 普通提示，分别使用配置里的铃声。
	notify.Success("计划任务成功", "每日备份已完成，耗时 3.2s")
	notify.Error("计划任务失败", "每日备份退出码 1，请检查日志")
	notify.Info("系统提示", "服务将于 10 分钟后重启")

	// 完整参数：级别、标题、内容、图标、跳转 URL、通知分组、是否静音。
	// NotifyAsync 在后台发送，网络异常不会阻塞业务。
	notify.NotifyAsync(
		notify.LevelError,
		"磁盘告警",
		"根分区使用率 95%",
		"https://example.com/icon.png",
		"https://panel.example.com",
		"2panel",
		false,
	)
}
