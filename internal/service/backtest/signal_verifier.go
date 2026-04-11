package backtest

import (
	"fmt"
	"math"
	"time"

	"github.com/smallfire/starfire/internal/config"
	"github.com/smallfire/starfire/internal/models"
	"github.com/smallfire/starfire/internal/service/strategy/helpers"
)

// VerifySignals 对回测产生的信号做后置验证（去重 + 正确性），只报告不过滤。
func VerifySignals(
	signals []*models.Signal,
	klines []models.Kline,
	strategyType string,
	cfg config.CandlestickStrategyConfig,
) *models.SignalVerificationReport {
	if len(signals) == 0 {
		return nil
	}

	report := &models.SignalVerificationReport{
		TotalSignals: len(signals),
		Results:      make([]models.SignalVerificationResult, len(signals)),
	}

	// 构建 CloseTime → 索引映射，用于快速定位 K 线
	closeTimeIndex := make(map[time.Time]int, len(klines))
	for i, k := range klines {
		closeTimeIndex[k.CloseTime] = i
	}

	// 去重检查
	type dedupKey struct {
		SymbolCode string
		SignalType string
		Period     string
		KlineTime  time.Time
	}
	seen := make(map[dedupKey]int) // key → first signal index
	for i, sig := range signals {
		result := &report.Results[i]
		result.SignalIndex = i
		result.SignalType = sig.SignalType
		if sig.KlineTime != nil {
			result.KlineTime = sig.KlineTime.Format("2006-01-02 15:04:05")
		}

		if sig.KlineTime == nil {
			result.Status = models.VerificationInvalid
			result.Reason = "kline_time 为空"
			report.InvalidCount++
			continue
		}

		// 去重
		key := dedupKey{sig.SymbolCode, sig.SignalType, sig.Period, *sig.KlineTime}
		if prevIdx, exists := seen[key]; exists {
			result.Status = models.VerificationDuplicate
			result.Reason = fmt.Sprintf("与第 %d 条信号重复", prevIdx)
			report.DuplicateCount++
			continue
		}
		seen[key] = i

		// 正确性检查
		klineIdx, found := closeTimeIndex[*sig.KlineTime]
		if !found {
			result.Status = models.VerificationInvalid
			result.Reason = "kline_time 在 K 线数据中未找到"
			report.InvalidCount++
			continue
		}

		status, reason := verifySignal(sig, klines, klineIdx, strategyType, cfg)
		result.Status = status
		result.Reason = reason
		switch status {
		case models.VerificationValid:
			report.ValidCount++
		case models.VerificationInvalid:
			report.InvalidCount++
		case models.VerificationSkipped:
			report.SkippedCount++
		}
	}

	return report
}

// verifySignal 按策略类型分发验证
func verifySignal(
	sig *models.Signal,
	klines []models.Kline,
	klineIdx int,
	strategyType string,
	cfg config.CandlestickStrategyConfig,
) (models.VerificationStatus, string) {
	switch strategyType {
	case "candlestick":
		return verifyCandlestickSignal(sig, klines, klineIdx, cfg)
	default:
		return models.VerificationSkipped, fmt.Sprintf("未实现 %s 策略的验证", strategyType)
	}
}

// verifyCandlestickSignal 验证 K 线形态信号
func verifyCandlestickSignal(
	sig *models.Signal,
	klines []models.Kline,
	klineIdx int,
	cfg config.CandlestickStrategyConfig,
) (models.VerificationStatus, string) {
	// 获取配置阈值（与策略使用的默认值一致）
	bodyATRThreshold := cfg.BodyATRThreshold
	if bodyATRThreshold <= 0 {
		bodyATRThreshold = 0.5
	}
	starBodyATRMax := cfg.StarBodyATRMax
	if starBodyATRMax <= 0 {
		starBodyATRMax = 0.3
	}
	midpointMin := cfg.StarMidpointMin
	if midpointMin <= 0 {
		midpointMin = 0.005
	}
	minCount := cfg.MomentumMinCount
	if minCount < 3 {
		minCount = 3
	}

	// 用 signal_data 中存储的 ATR 做验证（与策略生成时一致的 ATR）
	var storedATR float64
	if sig.SignalData != nil {
		if v, ok := (*sig.SignalData)["atr"].(float64); ok {
			storedATR = v
		}
	}
	atr := storedATR
	if atr <= 0 {
		return models.VerificationInvalid, "signal_data 中缺少有效的 atr 值"
	}

	switch sig.SignalType {
	case models.SignalTypeEngulfingBullish, models.SignalTypeEngulfingBearish:
		return checkEngulfingSignal(sig, klines, klineIdx, atr, bodyATRThreshold)
	case models.SignalTypeMomentumBullish, models.SignalTypeMomentumBearish:
		return checkMomentumSignal(sig, klines, klineIdx, atr, bodyATRThreshold, float64(minCount))
	case models.SignalTypeMorningStar, models.SignalTypeEveningStar:
		return checkStarSignal(sig, klines, klineIdx, atr, bodyATRThreshold, starBodyATRMax, midpointMin)
	default:
		return models.VerificationSkipped, fmt.Sprintf("未实现 %s 信号的验证", sig.SignalType)
	}
}

// checkEngulfingSignal 验证吞没形态信号
func checkEngulfingSignal(
	sig *models.Signal,
	klines []models.Kline,
	klineIdx int,
	atr, bodyATRThreshold float64,
) (models.VerificationStatus, string) {
	if klineIdx < 1 {
		return models.VerificationInvalid, "K 线索引不足，无法验证吞没形态（需要前一根 K 线）"
	}

	prev := klines[klineIdx-1]
	curr := klines[klineIdx]

	// 条件1: 两根 K 线方向必须相反
	if helpers.IsBullish(prev) == helpers.IsBullish(curr) {
		return models.VerificationInvalid, fmt.Sprintf("吞没形态要求方向相反，但两根 K 线同向")
	}

	// 条件2: 当前 K 线实体 >= bodyATRThreshold * ATR
	currBody := helpers.BodySize(curr)
	if currBody < atr*bodyATRThreshold {
		return models.VerificationInvalid, fmt.Sprintf("实体 %.4f 小于阈值 %.4f (ATR*%.1f)", currBody, atr*bodyATRThreshold, bodyATRThreshold)
	}

	// 条件3: 实体包含
	if helpers.IsBullish(curr) { // 阳包阴
		if !(curr.OpenPrice < prev.ClosePrice && curr.ClosePrice > prev.OpenPrice) {
			return models.VerificationInvalid, "阳包阴实体包含条件不满足"
		}
	} else { // 阴包阳
		if !(curr.OpenPrice > prev.ClosePrice && curr.ClosePrice < prev.OpenPrice) {
			return models.VerificationInvalid, "阴包阳实体包含条件不满足"
		}
	}

	// 交叉验证 signal_data
	if err := verifySignalDataValues(sig, map[string]float64{
		"prev_body_size":  helpers.BodySize(prev),
		"curr_body_size":  currBody,
		"curr_body_atr_ratio": currBody / atr,
	}); err != "" {
		return models.VerificationInvalid, err
	}

	return models.VerificationValid, ""
}

// checkMomentumSignal 验证动量信号（连 K 实体递增）
// 注意：回测使用滑动窗口，策略在窗口内看到连续同向 K 线的数量可能与全量数据不同，
// 因此验证器直接使用 signal_data 中策略存储的值做条件检查，不独立回溯 K 线序列。
func checkMomentumSignal(
	sig *models.Signal,
	klines []models.Kline,
	klineIdx int,
	atr, bodyATRThreshold, minCount float64,
) (models.VerificationStatus, string) {
	if sig.SignalData == nil {
		return models.VerificationInvalid, "signal_data 为空"
	}

	// 从 signal_data 读取策略存储的值
	count := toFloat((*sig.SignalData)["count"])
	firstBody := toFloat((*sig.SignalData)["first_body_size"])
	lastBody := toFloat((*sig.SignalData)["last_body_size"])
	bodyRatio := toFloat((*sig.SignalData)["body_ratio"])
	storedATR := toFloat((*sig.SignalData)["atr"])

	if count <= 0 || firstBody <= 0 || lastBody <= 0 || storedATR <= 0 {
		return models.VerificationInvalid, fmt.Sprintf("signal_data 字段不完整: count=%.0f, first=%.4f, last=%.4f, atr=%.4f", count, firstBody, lastBody, storedATR)
	}

	// 条件1: 连续同向 K 线数量在 3~5 之间
	if count > 5 {
		return models.VerificationInvalid, fmt.Sprintf("连续同向 %.0f 根，超过 5 根不应产生信号", count)
	}
	if count < minCount {
		return models.VerificationInvalid, fmt.Sprintf("连续同向 %.0f 根，不足最少 %.0f 根", count, minCount)
	}

	// 条件2: 实体大小合理（不能太小）
	if firstBody < storedATR*0.15 {
		return models.VerificationInvalid, fmt.Sprintf("首根实体 %.4f 小于阈值 %.4f (ATR*0.15)", firstBody, storedATR*0.15)
	}
	if lastBody < storedATR*0.15 {
		return models.VerificationInvalid, fmt.Sprintf("末根实体 %.4f 小于阈值 %.4f (ATR*0.15)", lastBody, storedATR*0.15)
	}

	// 条件3: 实体递增
	if lastBody <= firstBody {
		return models.VerificationInvalid, fmt.Sprintf("实体未递增: 首根 %.4f, 末根 %.4f", firstBody, lastBody)
	}

	// 条件4: signal_data 内部一致性（body_ratio 应等于 last/first）
	if bodyRatio > 0 {
		expectedRatio := lastBody / firstBody
		if expectedRatio != 0 && math.Abs(bodyRatio-expectedRatio)/math.Abs(expectedRatio) > 0.01 {
			return models.VerificationInvalid, fmt.Sprintf("body_ratio 不一致: 存储 %.6f, 计算 %.6f", bodyRatio, expectedRatio)
		}
	}

	// 条件5: 确认信号 K 线方向与信号类型一致
	isBull := helpers.IsBullish(klines[klineIdx])
	if isBull && sig.SignalType == models.SignalTypeMomentumBearish {
		return models.VerificationInvalid, "K 线为阳线但信号类型为看跌动量"
	}
	if !isBull && sig.SignalType == models.SignalTypeMomentumBullish {
		return models.VerificationInvalid, "K 线为阴线但信号类型为看涨动量"
	}

	return models.VerificationValid, ""
}

// checkStarSignal 验证早晨/黄昏之星信号
func checkStarSignal(
	sig *models.Signal,
	klines []models.Kline,
	klineIdx int,
	atr, bodyATRThreshold, starBodyATRMax, midpointMin float64,
) (models.VerificationStatus, string) {
	if klineIdx < 2 {
		return models.VerificationInvalid, "K 线索引不足，无法验证星形形态（需要 3 根 K 线）"
	}

	first := klines[klineIdx-2]
	star := klines[klineIdx-1]
	third := klines[klineIdx]

	firstBody := helpers.BodySize(first)
	starBody := helpers.BodySize(star)
	thirdBody := helpers.BodySize(third)

	var midpointRatio float64

	if helpers.IsBearish(first) && helpers.IsBullish(third) {
		// 早晨之星
		if firstBody < atr*bodyATRThreshold {
			return models.VerificationInvalid, fmt.Sprintf("第一根实体 %.4f 小于阈值 %.4f", firstBody, atr*bodyATRThreshold)
		}
		if starBody > atr*starBodyATRMax {
			return models.VerificationInvalid, fmt.Sprintf("中间 K 线实体 %.4f 超过上限 %.4f", starBody, atr*starBodyATRMax)
		}
		if thirdBody < atr*bodyATRThreshold {
			return models.VerificationInvalid, fmt.Sprintf("第三根实体 %.4f 小于阈值 %.4f", thirdBody, atr*bodyATRThreshold)
		}
		firstMidpoint := (first.OpenPrice + first.ClosePrice) / 2
		if third.ClosePrice <= firstMidpoint {
			return models.VerificationInvalid, "第三根收盘价未穿透第一根中点"
		}
		midpointRatio = (third.ClosePrice - firstMidpoint) / first.ClosePrice
		if midpointRatio < midpointMin {
			return models.VerificationInvalid, fmt.Sprintf("中点穿透比例 %.4f%% 低于最低 %.4f%%", midpointRatio*100, midpointMin*100)
		}
	} else if helpers.IsBullish(first) && helpers.IsBearish(third) {
		// 黄昏之星
		if firstBody < atr*bodyATRThreshold {
			return models.VerificationInvalid, fmt.Sprintf("第一根实体 %.4f 小于阈值 %.4f", firstBody, atr*bodyATRThreshold)
		}
		if starBody > atr*starBodyATRMax {
			return models.VerificationInvalid, fmt.Sprintf("中间 K 线实体 %.4f 超过上限 %.4f", starBody, atr*starBodyATRMax)
		}
		if thirdBody < atr*bodyATRThreshold {
			return models.VerificationInvalid, fmt.Sprintf("第三根实体 %.4f 小于阈值 %.4f", thirdBody, atr*bodyATRThreshold)
		}
		firstMidpoint := (first.OpenPrice + first.ClosePrice) / 2
		if third.ClosePrice >= firstMidpoint {
			return models.VerificationInvalid, "第三根收盘价未穿透第一根中点"
		}
		midpointRatio = (firstMidpoint - third.ClosePrice) / first.ClosePrice
		if midpointRatio < midpointMin {
			return models.VerificationInvalid, fmt.Sprintf("中点穿透比例 %.4f%% 低于最低 %.4f%%", midpointRatio*100, midpointMin*100)
		}
	} else {
		return models.VerificationInvalid, "第一根和第三根 K 线方向不满足星形形态要求"
	}

	// 交叉验证 signal_data
	if err := verifySignalDataValues(sig, map[string]float64{
		"first_body_atr":         firstBody / atr,
		"star_body_atr":          starBody / atr,
		"third_body_atr":         thirdBody / atr,
		"third_close_vs_first_midpoint": math.Abs(third.ClosePrice-(first.OpenPrice+first.ClosePrice)/2) / first.ClosePrice,
		"midpoint_ratio":         math.Abs(midpointRatio),
	}); err != "" {
		return models.VerificationInvalid, err
	}

	return models.VerificationValid, ""
}

// verifySignalDataValues 交叉验证信号中存储的数据与实际计算值是否一致
func verifySignalDataValues(sig *models.Signal, expected map[string]float64) string {
	if sig.SignalData == nil {
		return "signal_data 为空"
	}
	for key, expectedVal := range expected {
		raw, ok := (*sig.SignalData)[key]
		if !ok {
			return fmt.Sprintf("signal_data 缺少字段 %s", key)
		}
		// 兼容 int 和 float64 两种存储类型（Go map[string]interface{} 不做自动转换）
		var actualVal float64
		switch v := raw.(type) {
		case float64:
			actualVal = v
		case int:
			actualVal = float64(v)
		default:
			return fmt.Sprintf("signal_data.%s 类型异常: %T", key, raw)
		}
		// 允许 1% 的误差（浮点精度）
		if expectedVal != 0 && math.Abs(actualVal-expectedVal)/math.Abs(expectedVal) > 0.01 {
			return fmt.Sprintf("signal_data.%s 不一致: 存储 %.6f, 实际 %.6f", key, actualVal, expectedVal)
		}
	}
	return ""
}

// toFloat 将 interface{} 转为 float64，兼容 int 和 float64
func toFloat(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	default:
		return 0
	}
}
