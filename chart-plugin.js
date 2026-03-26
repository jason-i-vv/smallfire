/**
 * BoxWatcher Chart Plugin - 基于Lightweight Charts的K线图表插件
 * 支持箱体、趋势、阻力位三种模式
 */

class BoxWatcherChart {
    constructor(containerId, options = {}) {
        this.container = document.getElementById(containerId);

        // 默认配置
        const defaultOptions = {
            layout: {
                background: { type: 'solid', color: '#16213e' },
                textColor: '#d1d4dc',
            },
            grid: {
                vertLines: { color: '#2a2a4a' },
                horzLines: { color: '#2a2a4a' },
            },
            crosshair: {
                mode: LightweightCharts.CrosshairMode.Normal,
            },
            timeScale: {
                borderColor: '#2a2a4a',
                timeVisible: true,
                timezone: 'Asia/Shanghai', // 默认时区配置
            },
            rightPriceScale: {
                borderColor: '#2a2a4a',
            },
        };

        // 深度合并配置，确保 timeScale 的配置能保留
        this.options = this._deepMerge(defaultOptions, options);

        this.chart = null;
        this.candleSeries = null;
        this.series = {}; // 存储其他系列
    }

    /**
     * 深度合并对象
     */
    _deepMerge(target, source) {
        const result = { ...target };
        for (const key in source) {
            if (source[key] instanceof Object && key in target) {
                result[key] = this._deepMerge(target[key], source[key]);
            } else {
                result[key] = source[key];
            }
        }
        return result;
    }

    /**
     * 初始化图表
     */
    init() {
        this.chart = LightweightCharts.createChart(this.container, this.options);
        console.log('✅ 图表创建完成');

        // K线系列
        this.candleSeries = this.chart.addCandlestickSeries({
            upColor: '#00ff88',
            downColor: '#ff6b6b',
            borderUpColor: '#00ff88',
            borderDownColor: '#ff6b6b',
            lastValueVisible: false,
            priceLineVisible: false,
        });

        return this;
    }

    /**
     * 设置K线数据
     * @param {Array} candles - K线数据数组
     * @param {number} visibleCount - 默认显示最近多少根K线
     */
    setCandles(candles, visibleCount = 50) {
        if (!this.candleSeries) this.init();

        // 🔧 时区转换：将UTC时间戳转换为北京时间（加8小时）
        // 这样 Lightweight Charts 显示的就是北京时间了
        const adjustedCandles = candles.map(candle => ({
            ...candle,
            time: candle.time + 8 * 3600 // 加8小时（秒）
        }));

        this.candleSeries.setData(adjustedCandles);

        // 自动调整时间范围 - 只显示最近的数据
        if (adjustedCandles.length > 0) {
            const startIdx = Math.max(0, adjustedCandles.length - visibleCount);
            this.chart.timeScale().setVisibleRange({
                from: adjustedCandles[startIdx].time,
                to: adjustedCandles[adjustedCandles.length - 1].time
            });
        }
        return this;
    }

    /**
     * 添加均线
     * @param {string} name - 均线名称
     * @param {Array} data - 均线数据
     * @param {string} color - 颜色
     * @param {number} lineWidth - 线宽
     */
    addLine(name, data, color, lineWidth = 2) {
        if (!this.chart) this.init();

        // 如果已存在，先删除
        if (this.series[name]) {
            this.chart.removeSeries(this.series[name]);
        }

        // 🔧 时区转换：将UTC时间戳转换为北京时间（加8小时）
        const adjustedData = data.map(d => ({
            ...d,
            time: d.time + 8 * 3600 // 加8小时（秒）
        }));

        const series = this.chart.addLineSeries({
            color: color,
            lineWidth: lineWidth,
            crosshairMarkerVisible: true,
            lastValueVisible: false,
            priceLineVisible: false,
        });
        series.setData(adjustedData);
        this.series[name] = series;

        return this;
    }

    /**
     * 添加箱体（矩形区域）
     * 使用精确的K线时间点，只覆盖箱体范围内的K线
     * @param {number} high - 箱体高点
     * @param {number} low - 箱体低点
     * @param {number} startTime - 开始时间戳
     * @param {number} endTime - 结束时间戳
     */
    addBox(high, low, startTime, endTime) {
        if (!this.chart) this.init();

        // 🔧 时区转换：将UTC时间戳转换为北京时间
        const adjustedStartTime = startTime + 8 * 3600;
        const adjustedEndTime = endTime + 8 * 3600;

        // 获取K线数据
        const candles = this.candleSeries ? this.candleSeries.data() : [];
        if (candles.length === 0) return this;

        // 找到箱体范围内的K线
        const boxCandles = candles.filter(c => c.time >= adjustedStartTime && c.time <= adjustedEndTime);
        if (boxCandles.length === 0) return this;

        // 箱体顶线 - 只用箱体内的K线时间点
        const boxHigh = this.chart.addLineSeries({
            color: '#ff6b6b',
            lineWidth: 2,
            lineStyle: LightweightCharts.LineStyle.Solid,
            crosshairMarkerVisible: false,
            lastValueVisible: false,
            priceLineVisible: false,
        });
        boxHigh.setData(boxCandles.map(c => ({ time: c.time, value: high })));

        // 箱体底线
        const boxLow = this.chart.addLineSeries({
            color: '#00ff88',
            lineWidth: 2,
            lineStyle: LightweightCharts.LineStyle.Solid,
            crosshairMarkerVisible: false,
            lastValueVisible: false,
            priceLineVisible: false,
        });
        boxLow.setData(boxCandles.map(c => ({ time: c.time, value: low })));

        // 左边界线 - 从low到high的竖线
        const boxLeft = this.chart.addLineSeries({
            color: '#ffa500',
            lineWidth: 2,
            lineStyle: LightweightCharts.LineStyle.Solid,
            crosshairMarkerVisible: false,
            lastValueVisible: false,
            priceLineVisible: false,
        });
        // 在起始时间点创建多个价格点来模拟竖线
        const leftPoints = [];
        const priceStep = (high - low) / 20; // 分成20段
        for (let i = 0; i <= 20; i++) {
            leftPoints.push({ time: adjustedStartTime, value: low + priceStep * i });
        }
        boxLeft.setData(leftPoints);

        // 右边界线 - 从low到high的竖线
        const boxRight = this.chart.addLineSeries({
            color: '#ffa500',
            lineWidth: 2,
            lineStyle: LightweightCharts.LineStyle.Solid,
            crosshairMarkerVisible: false,
            lastValueVisible: false,
            priceLineVisible: false,
        });
        const rightPoints = [];
        for (let i = 0; i <= 20; i++) {
            rightPoints.push({ time: adjustedEndTime, value: low + priceStep * i });
        }
        boxRight.setData(rightPoints);

        this.series['boxHigh'] = boxHigh;
        this.series['boxLow'] = boxLow;
        this.series['boxLeft'] = boxLeft;
        this.series['boxRight'] = boxRight;

        return this;
    }

    /**
     * 添加阻力位/支撑位
     * @param {Array} levels - 价位数组
     * @param {string} type - 'resistance' 或 'support'
     */
    addLevels(levels, type = 'resistance') {
        if (!this.chart) this.init();
        if (!levels || levels.length === 0) return this;

        const color = type === 'resistance' ? '#ff6b6b' : '#00ff88';

        // 只显示最强的一个
        const level = levels[0];
        const series = this.chart.addLineSeries({
            color: color,
            lineWidth: type === 'resistance' ? 2 : 2,
            lineStyle: LightweightCharts.LineStyle.Dashed,
            crosshairMarkerVisible: false,
            lastValueVisible: false,
            priceLineVisible: false,
        });

        const candles = this.candleSeries ? this.candleSeries.data() : [];
        if (candles.length > 0) {
            series.setData([
                { time: candles[0].time, value: level },
                { time: candles[candles.length - 1].time, value: level }
            ]);
        }

        this.series[type] = series;
        return this;
    }

    /**
     * 设置信号标记
     * @param {Array} markers - 信号数组
     */
    setMarkers(markers) {
        if (!this.candleSeries) this.init();
        if (markers && markers.length > 0) {
            // 🔧 时区转换：将UTC时间戳转换为北京时间
            const adjustedMarkers = markers.map(marker => ({
                ...marker,
                time: marker.time + 8 * 3600
            }));
            this.candleSeries.setMarkers(adjustedMarkers);
        }
        return this;
    }

    /**
     * 添加成交量柱状图
     * @param {Array} data - 成交量数据 [{time, value, color}]
     * @param {string} name - 系列名称
     */
    addHistogram(name, data, options = {}) {
        if (!this.chart) this.init();

        // 如果已存在，先删除
        if (this.series[name]) {
            this.chart.removeSeries(this.series[name]);
        }

        const color = options.color || '#26a69a';
        const priceScaleId = options.priceScaleId || 'volume';

        // 添加副图系列
        const histogramSeries = this.chart.addHistogramSeries({
            color: color,
            priceFormat: {
                type: 'volume',
            },
            priceScaleId: priceScaleId,
        });

        // 🔧 时区转换：将UTC时间戳转换为北京时间
        const adjustedData = data.map(d => ({
            ...d,
            time: d.time + 8 * 3600
        }));

        // 设置数据
        histogramSeries.setData(adjustedData);

        // 设置副图位置
        if (this.chart.priceScale(priceScaleId)) {
            this.chart.priceScale(priceScaleId).applyOptions({
                scaleMargins: {
                    top: 0.7, // 副图占下半部分
                    bottom: 0,
                },
            });
        }

        this.series[name] = histogramSeries;
        return this;
    }

    /**
     * 响应式调整
     */
    resize() {
        if (this.chart) {
            this.chart.resize(this.container.clientWidth, this.container.clientHeight || 500);
        }
        return this;
    }

    /**
     * 销毁图表
     */
    destroy() {
        if (this.chart) {
            this.chart.remove();
            this.chart = null;
        }
    }

    /**
     * 获取图表实例
     */
    getChart() {
        return this.chart;
    }
}

/**
 * 创建箱体图表
 */
function createBoxChart(containerId, data) {
    const chart = new BoxWatcherChart(containerId);

    // K线
    chart.setCandles(data.candles);

    // 箱体
    if (data.box) {
        chart.addBox(data.box.high, data.box.low, data.box.start, data.box.end);
    }

    // 信号标记
    if (data.signals && data.signals.length > 0) {
        chart.setMarkers(data.signals);
    }

    // 响应式
    window.addEventListener('resize', () => chart.resize());

    return chart;
}

/**
 * 创建趋势图表
 */
function createTrendChart(containerId, data) {
    const chart = new BoxWatcherChart(containerId);

    // K线
    chart.setCandles(data.candles);

    // 均线
    if (data.ma7) chart.addLine('ma7', data.ma7, '#ffd700', 2);
    if (data.ma25) chart.addLine('ma25', data.ma25, '#ff8c00', 2);
    if (data.ma99) chart.addLine('ma99', data.ma99, '#ff4500', 2);

    // 信号标记
    if (data.signals && data.signals.length > 0) {
        chart.setMarkers(data.signals);
    }

    // 响应式
    window.addEventListener('resize', () => chart.resize());

    return chart;
}

/**
 * 创建阻力位图表
 */
function createResistanceChart(containerId, data) {
    const chart = new BoxWatcherChart(containerId);

    // K线
    chart.setCandles(data.candles);

    // 阻力位
    if (data.resistance && data.resistance.length > 0) {
        chart.addLevels(data.resistance, 'resistance');
    }

    // 支撑位
    if (data.support && data.support.length > 0) {
        chart.addLevels(data.support, 'support');
    }

    // 响应式
    window.addEventListener('resize', () => chart.resize());

    return chart;
}

// 导出到全局
window.BoxWatcherChart = BoxWatcherChart;
window.createBoxChart = createBoxChart;
window.createTrendChart = createTrendChart;
window.createResistanceChart = createResistanceChart;
