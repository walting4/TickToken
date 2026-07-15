// i18n 中英文翻译模块
// 支持 zh / en 两种语言，默认 zh

const translations = {
  zh: {
    // Header
    appTitle: 'TickToken',
    caCert: 'CA 证书',
    dataDir: '数据目录',
    language: '语言',
    passive: '被动模式',
    relay: '中继模式',

    // Summary cards
    promptTokens: 'Prompt Tokens',
    completionTokens: 'Completion Token',
    cacheHit: '缓存命中',
    totalEvents: '事件总数',

    // Verification section
    verificationStats: '校验统计',
    totalVerified: '总校验数',
    accuracyRate: '准确率',
    avgDeviation: '平均偏差',
    anomalies: '异常数',
    withResponseUsage: '响应 usage',
    withLocalOnly: '仅本地估算',
    avgLatency: '平均延迟',
    maxLatency: '最大延迟',

    // API Key monitor
    apikeyMonitor: 'API Key 监控',
    totalRequests: '请求总数',
    completedRequests: '已完成',
    failedRequests: '失败数',
    anomalyRequests: '异常数',
    inflightCount: '进行中',
    inflightRequests: '进行中',

    // Proxy config
    proxyConfig: '代理配置',
    oneClickSetup: '一键配置',
    removeProxy: '移除代理',
    systemProxy: '系统代理',
    traeProxy: 'trae 代理',
    proxyListening: '代理监听',
    captured: '已捕获',
    sseStreams: 'SSE 流',
    proxyErrors: '代理错误',
    configured: '已配置',
    notConfigured: '未配置',
    setupSuccess: '配置成功！请重启 trae/VSCode 使代理生效',
    setupPartial: '部分配置失败，请查看警告',
    removeSuccess: '代理已移除，请重启 trae/VSCode',
    caCertInstalled: 'CA 证书已安装',
    listening: '监听中',
    notListening: '未监听',
    yes: '是',
    no: '否',

    // Time series
    timeSeries: '时间序列',
    timeRange: '时间范围',
    hour1: '1 小时',
    hour6: '6 小时',
    hour24: '24 小时',
    day7: '7 天',
    refresh: '刷新',

    // Aggregate
    aggregate: '分类聚合',
    byModel: '按模型',
    byTool: '按工具',

    // Trend analysis
    trendAnalysis: '趋势分析',
    granularity: '粒度',
    hourly: '按小时',
    daily: '按天',
    totalTokens: '总 Token',
    eventCount: '事件数',
    anomalyCount: '异常数',

    // Anomaly events
    anomalyEvents: '异常事件',
    anomalyStats: '异常统计',
    anomalyRate: '异常率',
    noAnomalies: '暂无异常事件',
    loadMore: '加载更多',

    // Live events
    liveEvents: '实时事件',
    waitingEvents: '等待事件...',
    time: '时间',
    tool: '工具',
    model: '模型',
    source: '来源',

    // Anomaly types
    anomaly_none: '无',
    anomaly_prompt_deviation: 'Prompt 偏差',
    anomaly_completion_deviation: 'Completion 偏差',
    anomaly_zero_tokens: '零 Token',
    anomaly_missing_usage: '缺少 usage',
    anomaly_negative_tokens: '负数 Token',
    anomaly_latency_spike: '延迟飙升',
    anomaly_response_size_anomaly: '响应过大',

    // Units
    ms: 'ms',
    tokens: 'tokens',
    requests: '个请求',
  },
  en: {
    // Header
    appTitle: 'TickToken',
    caCert: 'CA Cert',
    dataDir: 'Data Dir',
    language: 'Language',
    passive: 'Passive',
    relay: 'Relay',

    // Summary cards
    promptTokens: 'Prompt Tokens',
    completionTokens: 'Completion Tokens',
    cacheHit: 'Cache Hit',
    totalEvents: 'Total Events',

    // Verification section
    verificationStats: 'Verification Stats',
    totalVerified: 'Total Verified',
    accuracyRate: 'Accuracy Rate',
    avgDeviation: 'Avg Deviation',
    anomalies: 'Anomalies',
    withResponseUsage: 'Response Usage',
    withLocalOnly: 'Local Only',
    avgLatency: 'Avg Latency',
    maxLatency: 'Max Latency',

    // API Key monitor
    apikeyMonitor: 'API Key Monitor',
    totalRequests: 'Total Requests',
    completedRequests: 'Completed',
    failedRequests: 'Failed',
    anomalyRequests: 'Anomalies',
    inflightCount: 'Inflight',
    inflightRequests: 'Inflight',

    // Proxy config
    proxyConfig: 'Proxy Config',
    oneClickSetup: 'One-Click Setup',
    removeProxy: 'Remove Proxy',
    systemProxy: 'System Proxy',
    traeProxy: 'Trae Proxy',
    proxyListening: 'Proxy Listening',
    captured: 'Captured',
    sseStreams: 'SSE Streams',
    proxyErrors: 'Proxy Errors',
    configured: 'Configured',
    notConfigured: 'Not Configured',
    setupSuccess: 'Setup complete! Please restart trae/VSCode',
    setupPartial: 'Some steps failed, check warnings',
    removeSuccess: 'Proxy removed, restart trae/VSCode',
    caCertInstalled: 'CA Cert Installed',
    listening: 'Listening',
    notListening: 'Not Listening',
    yes: 'Yes',
    no: 'No',

    // Time series
    timeSeries: 'Time Series',
    timeRange: 'Time Range',
    hour1: '1 hour',
    hour6: '6 hours',
    hour24: '24 hours',
    day7: '7 days',
    refresh: 'Refresh',

    // Aggregate
    aggregate: 'Aggregate',
    byModel: 'By Model',
    byTool: 'By Tool',

    // Trend analysis
    trendAnalysis: 'Trend Analysis',
    granularity: 'Granularity',
    hourly: 'Hourly',
    daily: 'Daily',
    totalTokens: 'Total Tokens',
    eventCount: 'Events',
    anomalyCount: 'Anomalies',

    // Anomaly events
    anomalyEvents: 'Anomaly Events',
    anomalyStats: 'Anomaly Stats',
    anomalyRate: 'Anomaly Rate',
    noAnomalies: 'No anomaly events',
    loadMore: 'Load More',

    // Live events
    liveEvents: 'Live Events',
    waitingEvents: 'Waiting for events...',
    time: 'Time',
    tool: 'Tool',
    model: 'Model',
    source: 'Source',

    // Anomaly types
    anomaly_none: 'none',
    anomaly_prompt_deviation: 'Prompt Deviation',
    anomaly_completion_deviation: 'Completion Deviation',
    anomaly_zero_tokens: 'Zero Tokens',
    anomaly_missing_usage: 'Missing Usage',
    anomaly_negative_tokens: 'Negative Tokens',
    anomaly_latency_spike: 'Latency Spike',
    anomaly_response_size_anomaly: 'Response Size Anomaly',

    // Units
    ms: 'ms',
    tokens: 'tokens',
    requests: 'requests',
  }
};

let currentLang = 'zh';

// 设置当前语言
function setLang(lang) {
  if (translations[lang]) {
    currentLang = lang;
  }
}

function getLang() {
  return currentLang;
}

// 翻译函数：t('key')
function t(key) {
  const dict = translations[currentLang] || translations.zh;
  return dict[key] !== undefined ? dict[key] : key;
}

// 获取异常类型的本地化名称
function tAnomaly(type) {
  if (!type) return t('anomaly_none');
  return t('anomaly_' + type) || type;
}

// 应用所有 data-i18n 标记的文本
function applyTranslations() {
  document.querySelectorAll('[data-i18n]').forEach(el => {
    const key = el.getAttribute('data-i18n');
    el.textContent = t(key);
  });
  document.querySelectorAll('[data-i18n-title]').forEach(el => {
    const key = el.getAttribute('data-i18n-title');
    el.title = t(key);
  });
  // 更新 html lang 属性
  document.documentElement.lang = currentLang;
}

window.i18n = {
  setLang,
  getLang,
  t,
  tAnomaly,
  applyTranslations,
  translations
};
