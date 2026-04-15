package market

// SyncHook K线同步完成后的回调钩子
type SyncHook interface {
	OnKlinesSynced(symbolID int, symbolCode, marketCode, period string)
}
