package network

// SystemProxy 是系统代理设置的只读快照，仅用于设置页展示；
// 请求实际走哪个代理仍由 netproxy 决定。
type SystemProxy struct {
	// Supported 报告当前平台是否可以读取系统代理设置。
	Supported bool
	// Address 是手动代理地址的展示文本；空表示未配置。
	Address string
	// Exceptions 是系统绕过列表的原始条目。
	Exceptions []string
	// AutoConfig 是 PAC 脚本地址；非空表示代理由脚本决定。
	AutoConfig string
	// AutoDetect 表示系统开启了自动检测（WPAD）。
	AutoDetect bool
}
