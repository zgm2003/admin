(function () {
  'use strict';
  var replacements = [
    ['Asynq - Monitoring', 'Asynq - 任务队列监控'], ['Asynq monitoring web console', 'Asynq 任务队列监控控制台'],
    ['Queues', '队列'], ['Servers', '服务器'], ['Schedulers', '调度器'], ['Metrics', '指标'], ['Settings', '设置'],
    ['Dashboard', '仪表盘'], ['Tasks', '任务'], ['Task Info', '任务信息'], ['Pending', '待处理'], ['Active', '执行中'],
    ['Scheduled', '已调度'], ['Retry', '重试中'], ['Archived', '已归档'], ['Completed', '已完成'], ['Refresh', '刷新'],
    ['Details', '详情'], ['Status', '状态'], ['Loading', '加载中'], ['Error', '错误'], ['Cancel', '取消'], ['Delete', '删除'],
    ['Pause', '暂停'], ['Resume', '恢复'], ['Run', '运行'], ['Archive', '归档'], ['Close', '关闭'], ['No data', '暂无数据'],
    ['Max Retry', '最大重试次数'], ['Read Only', '只读模式'], ['Read-only', '只读模式'], ['Task Type', '任务类型'],
    ['Payload', '负载'], ['Result', '结果'], ['Latency', '延迟'], ['Memory Usage', '内存使用']
  ];
  function translate(value) {
    if (typeof value !== 'string') return value;
    replacements.forEach(function (item) { value = value.split(item[0]).join(item[1]); });
    return value;
  }
  function translateNode(node) {
    if (node.nodeType === Node.TEXT_NODE) { var translated = translate(node.nodeValue); if (translated !== node.nodeValue) node.nodeValue = translated; return; }
    if (node.nodeType !== Node.ELEMENT_NODE) return;
    ['aria-label', 'title', 'alt'].forEach(function (name) {
      if (node.hasAttribute(name)) node.setAttribute(name, translate(node.getAttribute(name)));
    });
    node.childNodes.forEach(translateNode);
  }
  function apply() { if (document.body) translateNode(document.body); }
  new MutationObserver(apply).observe(document.documentElement, { childList: true, subtree: true, characterData: true });
  document.addEventListener('DOMContentLoaded', apply);
}());
