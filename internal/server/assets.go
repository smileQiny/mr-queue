package server

const indexHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>mr-queue</title>
  <style>
    :root {
      --bg: #f6f7f9;
      --panel: #ffffff;
      --text: #17202a;
      --muted: #667085;
      --line: #d8dee8;
      --accent: #0f766e;
      --accent-dark: #115e59;
      --warn: #b45309;
      --bad: #b42318;
      --good: #067647;
      --shadow: 0 12px 30px rgba(15, 23, 42, 0.08);
    }

    * { box-sizing: border-box; }
    body {
      margin: 0;
      font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      background: var(--bg);
      color: var(--text);
    }

    header {
      min-height: 72px;
      border-bottom: 1px solid var(--line);
      background: rgba(255,255,255,0.92);
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 18px;
      padding: 0 28px;
      position: sticky;
      top: 0;
      z-index: 2;
      backdrop-filter: blur(10px);
    }

    h1 {
      font-size: 20px;
      line-height: 1.2;
      margin: 0;
      font-weight: 720;
      letter-spacing: 0;
    }

    .sub {
      color: var(--muted);
      font-size: 13px;
      margin-top: 4px;
    }

    .actions {
      display: flex;
      align-items: center;
      gap: 10px;
      flex-wrap: wrap;
      justify-content: flex-end;
    }

    .loop-options {
      display: grid;
      grid-template-columns: repeat(5, minmax(76px, 1fr));
      gap: 8px;
      width: 100%;
      max-width: 540px;
    }

    .loop-options label {
      display: grid;
      gap: 3px;
      color: var(--muted);
      font-size: 11px;
    }

    input {
      appearance: none;
      border: 1px solid var(--line);
      background: #fff;
      color: var(--text);
      border-radius: 6px;
      min-height: 32px;
      padding: 0 8px;
      font: inherit;
      font-size: 13px;
      min-width: 0;
    }

    button {
      appearance: none;
      border: 1px solid var(--line);
      background: #fff;
      color: var(--text);
      border-radius: 6px;
      min-height: 36px;
      padding: 0 12px;
      font: inherit;
      font-size: 14px;
      cursor: pointer;
    }

    button.primary {
      background: var(--accent);
      border-color: var(--accent);
      color: #fff;
    }

    button.primary:hover { background: var(--accent-dark); }
    button:disabled { opacity: .55; cursor: not-allowed; }

    main {
      max-width: 1180px;
      margin: 0 auto;
      padding: 24px;
      display: grid;
      grid-template-columns: minmax(0, 1.7fr) minmax(320px, .9fr);
      gap: 18px;
    }

    section {
      background: var(--panel);
      border: 1px solid var(--line);
      border-radius: 8px;
      box-shadow: var(--shadow);
      min-width: 0;
    }

    .section-head {
      padding: 16px 18px;
      border-bottom: 1px solid var(--line);
      display: flex;
      justify-content: space-between;
      align-items: center;
      gap: 12px;
    }

    h2 {
      margin: 0;
      font-size: 15px;
      letter-spacing: 0;
    }

    .count {
      color: var(--muted);
      font-size: 13px;
      white-space: nowrap;
    }

    .queue {
      display: grid;
    }

    .task {
      display: grid;
      grid-template-columns: 132px minmax(0, 1fr) 110px;
      gap: 12px;
      padding: 15px 18px;
      border-bottom: 1px solid var(--line);
      align-items: start;
    }

    .task:last-child { border-bottom: 0; }

    .sha {
      font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
      font-size: 12px;
      color: #475467;
      padding-top: 2px;
      overflow-wrap: anywhere;
    }

    .subject {
      font-size: 14px;
      font-weight: 650;
      margin-bottom: 7px;
      overflow-wrap: anywhere;
    }

    .meta {
      color: var(--muted);
      font-size: 12px;
      line-height: 1.6;
      overflow-wrap: anywhere;
    }

    .status {
      justify-self: end;
      border-radius: 999px;
      padding: 4px 9px;
      border: 1px solid var(--line);
      font-size: 12px;
      line-height: 1.2;
      background: #f8fafc;
      color: #475467;
      white-space: nowrap;
    }

    .status.merged { color: var(--good); background: #ecfdf3; border-color: #abefc6; }
    .status.skipped { color: #175cd3; background: #eff8ff; border-color: #b2ddff; }
    .status.failed { color: var(--bad); background: #fef3f2; border-color: #fecdca; }
    .status.running, .status.mr_open, .status.reviewed, .status.pushed { color: var(--warn); background: #fffaeb; border-color: #fedf89; }

    .side {
      display: grid;
      gap: 18px;
      align-content: start;
    }

    .kv, .logs {
      padding: 14px 18px 18px;
    }

    .kv-row {
      display: grid;
      grid-template-columns: 98px minmax(0, 1fr);
      gap: 10px;
      padding: 8px 0;
      border-bottom: 1px solid #eef1f6;
      font-size: 13px;
    }

    .kv-row:last-child { border-bottom: 0; }
    .key { color: var(--muted); }
    .value { overflow-wrap: anywhere; }

    .log {
      padding: 10px 0;
      border-bottom: 1px solid #eef1f6;
      font-size: 13px;
      line-height: 1.45;
    }

    .log:last-child { border-bottom: 0; }
    .log-step { font-weight: 700; margin-right: 6px; }
    .log-time { color: var(--muted); font-size: 12px; margin-top: 4px; }

    .empty {
      padding: 34px 18px;
      text-align: center;
      color: var(--muted);
      font-size: 14px;
    }

    @media (max-width: 860px) {
      header { align-items: flex-start; flex-direction: column; padding: 16px 18px; }
      .actions { justify-content: flex-start; }
      .loop-options { grid-template-columns: repeat(2, minmax(0, 1fr)); max-width: none; }
      main { grid-template-columns: 1fr; padding: 16px; }
      .task { grid-template-columns: 1fr; }
      .status { justify-self: start; }
    }
  </style>
</head>
<body>
  <header>
    <div>
      <h1>mr-queue</h1>
      <div class="sub" id="subtitle">MR 队列编排器</div>
    </div>
    <div class="actions">
      <div class="loop-options">
        <label>最小间隔<input id="loopDelayMin" value="1m"></label>
        <label>最大间隔<input id="loopDelayMax" value="5m"></label>
        <label>开始时间<input id="workStart" type="time" value="08:00"></label>
        <label>结束时间<input id="workEnd" type="time" value="23:30"></label>
        <label>合入上限<input id="maxMerged" type="number" min="0" step="1" value="3"></label>
      </div>
      <button id="syncBtn" onclick="post('/api/sync-queue')">同步队列</button>
      <button class="primary" id="runBtn" onclick="post('/api/run-once')">运行下一条</button>
      <button id="loopBtn" onclick="runLoop()">自动运行</button>
      <button id="stopBtn" onclick="post('/api/stop')">停止</button>
      <button id="pauseBtn" onclick="post('/api/pause')">暂停</button>
      <button id="resumeBtn" onclick="post('/api/resume')">继续</button>
    </div>
  </header>
  <main>
    <section>
      <div class="section-head">
        <h2>Commit 队列</h2>
        <div class="count" id="count">0 条</div>
      </div>
      <div class="queue" id="queue"></div>
    </section>
    <div class="side">
      <section>
        <div class="section-head"><h2>配置</h2><span class="count">token 已隐藏</span></div>
        <div class="kv" id="config"></div>
      </section>
      <section>
        <div class="section-head"><h2>最新日志</h2><span class="count" id="running">idle</span></div>
        <div class="logs" id="logs"></div>
      </section>
    </div>
  </main>
  <script>
    async function post(path) {
      await fetch(path, { method: 'POST' });
      await refresh();
    }

    async function retry(sha) {
      await fetch('/api/retry?sha=' + encodeURIComponent(sha), { method: 'POST' });
      await refresh();
    }

    async function runLoop() {
      const params = new URLSearchParams({
        loop_delay_min: document.getElementById('loopDelayMin').value,
        loop_delay_max: document.getElementById('loopDelayMax').value,
        work_window_start: document.getElementById('workStart').value,
        work_window_end: document.getElementById('workEnd').value,
        max_merged_commits: document.getElementById('maxMerged').value
      });
      await fetch('/api/run-loop?' + params.toString(), { method: 'POST' });
      await refresh();
    }

    async function refresh() {
      const res = await fetch('/api/status');
      const data = await res.json();
      const tasks = Object.values(data.state.tasks || {}).sort(compareTasks);
      document.getElementById('count').textContent = tasks.length + ' 条';
      document.getElementById('running').textContent = data.running ? 'running' : 'idle';
      document.getElementById('syncBtn').disabled = data.running || data.state.paused;
      document.getElementById('runBtn').disabled = data.running || data.state.paused;
      document.getElementById('loopBtn').disabled = data.running || data.state.paused;
      document.getElementById('stopBtn').disabled = !data.running;
      for (const id of ['loopDelayMin', 'loopDelayMax', 'workStart', 'workEnd', 'maxMerged']) {
        document.getElementById(id).disabled = data.running;
      }
      document.getElementById('pauseBtn').disabled = data.state.paused;
      document.getElementById('resumeBtn').disabled = !data.state.paused;
      document.getElementById('subtitle').textContent = data.state.paused ? '已暂停' : (data.running ? ('正在运行：' + (data.mode || 'once')) : 'MR 队列编排器');
      applyLoopDefaults(data.config || {});
      renderQueue(tasks);
      renderConfig(data.config || {});
      renderLogs(tasks, data.lastErr);
    }

    function applyLoopDefaults(config) {
      if (window.loopDefaultsApplied) return;
      const workflow = config.workflow || {};
      document.getElementById('loopDelayMin').value = workflow.loop_delay_min || '1m';
      document.getElementById('loopDelayMax').value = workflow.loop_delay_max || workflow.loop_delay_min || '5m';
      window.loopDefaultsApplied = true;
    }

    function renderQueue(tasks) {
      const el = document.getElementById('queue');
      if (!tasks.length) {
        el.innerHTML = '<div class="empty">还没有任务。点击“同步队列”只加载 commit 列表；点击“运行下一条”会执行第一个待处理 MR。</div>';
        return;
      }
      el.innerHTML = tasks.map(task => {
        const retryButton = task.status === 'failed'
          ? '<button onclick="retry(' + JSON.stringify(task.sha).replaceAll('"', '&quot;') + ')">重试</button>'
          : '';
        const mr = task.mr_url
          ? '<a href="' + escapeHTML(task.mr_url) + '" target="_blank">MR #' + task.mr_number + '</a>'
          : (task.mr_number ? 'MR #' + escapeHTML(task.mr_number) : 'MR 未创建');
        const shaMap = '队列：' + escapeHTML(shortSha(task.sha)) +
          ' -> MR：' + escapeHTML(shortSha(task.mr_commit_sha) || '-') +
          ' -> 社区：' + escapeHTML(shortSha(task.community_commit_sha) || '-');
        return '<div class="task">' +
          '<div class="sha">#' + escapeHTML(displayIndex(task)) + '<br>' + escapeHTML(shortSha(task.sha)) + '</div>' +
          '<div><div class="subject">' + escapeHTML(task.subject || '(no subject)') + '</div>' +
          '<div class="meta">映射：' + shaMap + '<br>分支：' + escapeHTML(task.branch || '-') + '<br>' + mr + '<br>错误：' + escapeHTML(task.error || '-') + '</div>' +
          '</div>' +
          '<div><div class="status ' + escapeHTML(task.status || '') + '">' + escapeHTML(task.status || 'pending') + '</div><div style="margin-top:8px">' + retryButton + '</div></div>' +
          '</div>';
      }).join('');
    }

    function renderConfig(config) {
      const rows = [
        ['本地目录', (config.local && config.local.path) || (config.source && config.source.local_path)],
        ['commit_range', config.workflow && config.workflow.commit_range],
        ['起止 commit', commitBounds(config)],
        ['映射关系', mappingText(config)],
        ['队列分支', config.queue ? (config.queue.remote + '/' + config.queue.branch) : ''],
        ['队列基线', config.queue && config.queue.base_ref],
        ['MR 分支', config.private ? (config.private.remote + '/' + branchTemplateText(config.private)) : ''],
        ['目标仓库', config.community ? (config.community.owner + '/' + config.community.repo) : ''],
        ['目标分支', config.community && config.community.branch],
        ['合并方式', config.workflow && config.workflow.merge_method],
        ['等待评论', config.workflow && config.workflow.required_comment_text],
        ['自动间隔', loopDelayText(config.workflow)],
        ['本轮限制', '页面启动时设置'],
        ['提交账号', config.auth && config.auth.submitter && config.auth.submitter.token_env],
        ['审核账号', config.auth && config.auth.reviewer && config.auth.reviewer.token_env],
        ['合并账号', config.auth && config.auth.maintainer && config.auth.maintainer.token_env]
      ];
      document.getElementById('config').innerHTML = rows.map(([k, v]) =>
        '<div class="kv-row"><div class="key">' + escapeHTML(k) + '</div><div class="value">' + escapeHTML(v || '-') + '</div></div>'
      ).join('');
    }

    function renderLogs(tasks, lastErr) {
      const logs = [];
      for (const task of tasks) {
        for (const log of (task.logs || [])) {
          if (log.step === 'error' && task.status !== 'failed') continue;
          logs.push({ ...log, sha: task.sha, taskStatus: task.status, queueIndex: task.queue_index });
        }
      }
      logs.sort((a, b) => (b.time || '').localeCompare(a.time || ''));
      if (lastErr) logs.unshift({ step: 'last error', message: lastErr, time: '' });
      const el = document.getElementById('logs');
      if (!logs.length) {
        el.innerHTML = '<div class="empty">暂无日志</div>';
        return;
      }
      el.innerHTML = logs.slice(0, 12).map(log =>
        '<div class="log"><span class="log-step">' + escapeHTML(log.step) + '</span>' + escapeHTML(displayLogMessage(log)) +
        '<div class="log-time">' + escapeHTML(log.time || '') + '</div></div>'
      ).join('');
    }

    function shortSha(sha) { return (sha || '').slice(0, 12); }
    function compareTasks(a, b) {
      return queueIndex(a) - queueIndex(b) ||
        (a.created_at || '').localeCompare(b.created_at || '') ||
        (a.sha || '').localeCompare(b.sha || '');
    }
    function queueIndex(task) {
      return Number.isFinite(task.queue_index) ? task.queue_index : Number.MAX_SAFE_INTEGER;
    }
    function displayIndex(task) {
      const index = queueIndex(task);
      return index === Number.MAX_SAFE_INTEGER ? '-' : String(index + 1);
    }
    function commitBounds(config) {
      if (!config.queue) return '-';
      return (config.queue.start_sha || '-') + ' -> ' + (config.queue.end_sha || '-');
    }
    function mappingText(config) {
      const queue = config.queue ? (config.queue.remote + '/' + config.queue.branch) : '-';
      const base = config.queue && config.queue.base_ref ? config.queue.base_ref : '-';
      const mr = config.private ? (config.private.remote + '/' + branchTemplateText(config.private)) : '-';
      const target = config.community ? (config.community.owner + '/' + config.community.repo + ':' + config.community.branch) : '-';
      return base + '..' + queue + ' -> ' + mr + ' -> ' + target;
    }
    function branchTemplateText(privateConfig) {
      const prefix = privateConfig.branch_prefix || 'mr-queue';
      return (privateConfig.branch_template || '{prefix}-{sha12}').replaceAll('{prefix}', prefix);
    }
    function loopDelayText(workflow) {
      if (!workflow) return '-';
      return (workflow.loop_delay_min || '-') + ' .. ' + (workflow.loop_delay_max || workflow.loop_delay_min || '-');
    }
    function displayLogMessage(log) {
      const message = String(log.message || '');
      if (log.step === 'approval' && message.includes('Approval failed but continuing')) {
        return 'Approval was rejected by the platform; continued because approval_failure_mode=warn';
      }
      return message;
    }
    function escapeHTML(value) {
      return String(value == null ? '' : value).replace(/[&<>"']/g, ch => ({
        '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'
      }[ch]));
    }

    refresh();
    setInterval(refresh, 2000);
  </script>
</body>
</html>`
