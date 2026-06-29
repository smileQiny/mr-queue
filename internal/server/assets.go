package server

const indexHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <link rel="icon" href="data:," />
  <title>mr-queue</title>
  <style>
    :root {
      --bg: #f4f6f8;
      --surface: #ffffff;
      --surface-muted: #f8fafc;
      --text: #121926;
      --muted: #667085;
      --faint: #98a2b3;
      --line: #d9e0ea;
      --line-soft: #edf1f6;
      --accent: #0f766e;
      --accent-dark: #0b5c56;
      --blue: #2563eb;
      --warn: #b54708;
      --bad: #b42318;
      --good: #067647;
      --shadow: 0 14px 38px rgba(18, 25, 38, 0.08);
      --shadow-soft: 0 1px 0 rgba(18, 25, 38, 0.04);
    }

    * { box-sizing: border-box; }

    [hidden] { display: none !important; }

    body {
      margin: 0;
      font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      background:
        linear-gradient(180deg, #eef4f3 0, #f4f6f8 260px),
        var(--bg);
      color: var(--text);
    }

    a {
      color: var(--blue);
      text-decoration: none;
    }

    a:hover { text-decoration: underline; }

    header {
      min-height: 76px;
      border-bottom: 1px solid rgba(217, 224, 234, 0.92);
      background: rgba(255, 255, 255, 0.9);
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 18px;
      padding: 14px 28px;
      position: sticky;
      top: 0;
      z-index: 2;
      backdrop-filter: blur(12px);
    }

    .brand {
      display: flex;
      align-items: center;
      gap: 12px;
      min-width: 0;
    }

    .brand-mark {
      width: 40px;
      height: 40px;
      border-radius: 8px;
      display: grid;
      place-items: center;
      background: #12323a;
      color: #fff;
      font-size: 12px;
      font-weight: 800;
      letter-spacing: 0;
      box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.15);
      flex: 0 0 auto;
    }

    h1 {
      font-size: 20px;
      line-height: 1.2;
      margin: 0;
      font-weight: 760;
      letter-spacing: 0;
    }

    .sub {
      color: var(--muted);
      font-size: 13px;
      margin-top: 4px;
    }

    .header-meta {
      display: flex;
      align-items: center;
      justify-content: flex-end;
      gap: 10px;
      flex-wrap: wrap;
    }

    .state-badge,
    .header-count,
    .count {
      border: 1px solid var(--line);
      background: rgba(255, 255, 255, 0.82);
      color: var(--muted);
      border-radius: 999px;
      min-height: 30px;
      padding: 6px 10px;
      display: inline-flex;
      align-items: center;
      gap: 7px;
      font-size: 12px;
      line-height: 1;
      white-space: nowrap;
    }

    .state-dot {
      width: 7px;
      height: 7px;
      border-radius: 999px;
      background: var(--faint);
      box-shadow: 0 0 0 3px rgba(152, 162, 179, 0.14);
      flex: 0 0 auto;
    }

    .state-badge.running {
      color: var(--warn);
      border-color: #fedf89;
      background: #fffaeb;
    }

    .state-badge.running .state-dot {
      background: var(--warn);
      box-shadow: 0 0 0 3px rgba(181, 71, 8, 0.16);
    }

    .state-badge.paused {
      color: #175cd3;
      border-color: #b2ddff;
      background: #eff8ff;
    }

    .state-badge.paused .state-dot {
      background: #175cd3;
      box-shadow: 0 0 0 3px rgba(23, 92, 211, 0.14);
    }

    .state-badge.offline {
      color: var(--bad);
      border-color: #fecdca;
      background: #fef3f2;
    }

    .state-badge.offline .state-dot {
      background: var(--bad);
      box-shadow: 0 0 0 3px rgba(180, 35, 24, 0.14);
    }

    .state-badge.idle .state-dot {
      background: var(--good);
      box-shadow: 0 0 0 3px rgba(6, 118, 71, 0.14);
    }

    main {
      max-width: 1280px;
      margin: 0 auto;
      padding: 24px;
      display: grid;
      grid-template-columns: minmax(0, 1.42fr) minmax(360px, 0.92fr);
      gap: 18px;
    }

    section {
      background: rgba(255, 255, 255, 0.94);
      border: 1px solid var(--line);
      border-radius: 8px;
      box-shadow: var(--shadow);
      min-width: 0;
    }

    .overview-panel {
      grid-column: 1 / -1;
      display: grid;
      grid-template-columns: minmax(160px, 0.68fr) minmax(0, 1fr) 36px minmax(0, 1fr) 36px minmax(0, 1fr);
      gap: 0;
      overflow: hidden;
    }

    .overview-item {
      min-width: 0;
      padding: 15px 18px;
      border-right: 1px solid var(--line-soft);
    }

    .overview-item:last-child { border-right: 0; }

    .overview-item.compact {
      background: #fbfcfe;
    }

    .flow-arrow {
      display: grid;
      place-items: center;
      color: var(--faint);
      background: #fbfcfe;
      border-right: 1px solid var(--line-soft);
      font-size: 18px;
    }

    .overview-label {
      color: var(--faint);
      font-size: 11px;
      font-weight: 700;
      letter-spacing: 0;
      margin-bottom: 8px;
    }

    .overview-value {
      color: var(--text);
      font-size: 14px;
      font-weight: 760;
      line-height: 1.35;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .overview-sub {
      color: var(--muted);
      font-size: 12px;
      line-height: 1.35;
      margin-top: 5px;
      overflow-wrap: anywhere;
    }

    .overview-sub.single-line {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .control-panel {
      grid-column: 1 / -1;
      display: grid;
      grid-template-columns: minmax(0, 1fr);
      gap: 14px;
      padding: 15px 18px;
    }

    .control-title {
      grid-column: 1 / -1;
      display: flex;
      align-items: baseline;
      justify-content: space-between;
      gap: 12px;
      min-width: 0;
    }

    .control-title h2,
    .section-title h2 {
      margin: 0;
      font-size: 15px;
      letter-spacing: 0;
    }

    .control-summary,
    .section-sub {
      color: var(--muted);
      font-size: 12px;
      margin-top: 5px;
      line-height: 1.35;
    }

    .control-summary {
      margin-top: 0;
      text-align: right;
      overflow-wrap: anywhere;
    }

    .actions {
      display: flex;
      align-items: stretch;
      gap: 8px;
      flex-wrap: wrap;
      justify-content: flex-end;
    }

    .actions-primary,
    .actions-state {
      display: flex;
      align-items: stretch;
      gap: 8px;
      flex-wrap: wrap;
    }

    .actions-state {
      padding-left: 8px;
      border-left: 1px solid var(--line-soft);
    }

    .button-icon {
      font-size: 13px;
      line-height: 1;
    }

    .loop-options {
      display: grid;
      grid-template-columns: repeat(7, minmax(0, 1fr));
      gap: 8px;
      width: 100%;
    }

    .loop-options label {
      display: grid;
      gap: 5px;
      color: var(--muted);
      font-size: 11px;
      min-width: 0;
    }

    .loop-options input {
      width: 100%;
    }

    input, select {
      appearance: none;
      border: 1px solid var(--line);
      background: #fff;
      color: var(--text);
      border-radius: 6px;
      min-height: 34px;
      padding: 0 9px;
      font: inherit;
      font-size: 13px;
      min-width: 0;
      outline: none;
      transition: border-color 160ms ease, box-shadow 160ms ease, background 160ms ease;
    }

    input:focus, select:focus {
      border-color: rgba(15, 118, 110, 0.72);
      box-shadow: 0 0 0 3px rgba(15, 118, 110, 0.12);
    }

    select {
      padding-right: 26px;
      background-image: linear-gradient(45deg, transparent 50%, var(--muted) 50%), linear-gradient(135deg, var(--muted) 50%, transparent 50%);
      background-position: calc(100% - 14px) 14px, calc(100% - 9px) 14px;
      background-size: 5px 5px, 5px 5px;
      background-repeat: no-repeat;
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
      font-size: 13px;
      cursor: pointer;
      display: inline-flex;
      align-items: center;
      justify-content: center;
      gap: 6px;
      white-space: nowrap;
      transition: transform 120ms ease, border-color 160ms ease, background 160ms ease, color 160ms ease, box-shadow 160ms ease;
    }

    button:hover:not(:disabled) {
      border-color: #bdc7d5;
      background: var(--surface-muted);
      box-shadow: var(--shadow-soft);
    }

    button:active:not(:disabled) {
      transform: translateY(1px);
    }

    button.primary {
      background: var(--accent);
      border-color: var(--accent);
      color: #fff;
    }

    button.primary:hover:not(:disabled) {
      background: var(--accent-dark);
      border-color: var(--accent-dark);
    }

    button.danger {
      color: var(--bad);
      border-color: #fecdca;
      background: #fffafa;
    }

    button:disabled {
      opacity: .52;
      cursor: not-allowed;
      box-shadow: none;
    }

    .section-head {
      padding: 15px 18px;
      border-bottom: 1px solid var(--line);
      background: linear-gradient(180deg, #ffffff 0, #fbfcfe 100%);
      display: flex;
      justify-content: space-between;
      align-items: center;
      gap: 12px;
    }

    .queue-panel {
      height: min(780px, calc(100vh - 244px));
      min-height: 600px;
      display: grid;
      grid-template-rows: auto minmax(0, 1fr) auto;
    }

    .queue-head {
      align-items: center;
    }

    .queue-tools {
      display: flex;
      align-items: center;
      gap: 8px;
      flex-wrap: wrap;
      justify-content: flex-end;
    }

    .queue-summary {
      display: flex;
      align-items: center;
      gap: 6px;
      flex-wrap: wrap;
      justify-content: flex-end;
    }

    .page-size {
      display: flex;
      align-items: center;
      gap: 7px;
      color: var(--muted);
      font-size: 12px;
      white-space: nowrap;
    }

    .page-size select {
      width: 78px;
      min-height: 30px;
      font-size: 12px;
      background-position: calc(100% - 14px) 12px, calc(100% - 9px) 12px;
    }

    .queue {
      display: grid;
      align-content: start;
      min-height: 0;
      overflow: auto;
    }

    .task {
      display: grid;
      grid-template-columns: 108px minmax(0, 1fr) 116px;
      gap: 14px;
      padding: 15px 18px;
      border-bottom: 1px solid var(--line-soft);
      align-items: start;
      background: #fff;
      transition: background 160ms ease;
    }

    .task:hover {
      background: #fbfdff;
    }

    .task:last-child { border-bottom: 0; }

    .task-id {
      display: grid;
      gap: 7px;
      min-width: 0;
    }

    .task-number {
      color: var(--text);
      font-weight: 760;
      font-size: 13px;
      line-height: 1.2;
    }

    .sha,
    .mono {
      font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
      font-size: 12px;
    }

    .sha {
      color: #475467;
      overflow-wrap: anywhere;
    }

    .task-main {
      min-width: 0;
    }

    .subject {
      font-size: 14px;
      font-weight: 700;
      line-height: 1.4;
      margin-bottom: 8px;
      overflow-wrap: anywhere;
    }

    .task-meta {
      display: grid;
      gap: 4px;
      color: var(--muted);
      font-size: 12px;
      line-height: 1.5;
      overflow-wrap: anywhere;
    }

    .meta-row {
      display: grid;
      grid-template-columns: 48px minmax(0, 1fr);
      gap: 8px;
      min-width: 0;
    }

    .meta-label {
      color: var(--faint);
    }

    .task-error {
      color: var(--bad);
    }

    .task-state {
      display: grid;
      justify-items: end;
      gap: 8px;
    }

    .retry-action {
      min-height: 30px;
      padding: 0 10px;
      font-size: 12px;
    }

    .queue-footer {
      min-height: 54px;
      padding: 10px 18px;
      border-top: 1px solid var(--line);
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 12px;
      background: #fbfcfe;
      border-radius: 0 0 8px 8px;
    }

    .page-info {
      color: var(--muted);
      font-size: 13px;
      white-space: nowrap;
    }

    .pager {
      display: flex;
      align-items: center;
      gap: 7px;
    }

    .pager button {
      min-width: 36px;
      min-height: 32px;
      padding: 0 10px;
    }

    .status {
      justify-self: end;
      border-radius: 999px;
      padding: 5px 10px;
      border: 1px solid var(--line);
      font-size: 12px;
      line-height: 1;
      background: #f8fafc;
      color: #475467;
      white-space: nowrap;
      max-width: 100%;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    .status.merged { color: var(--good); background: #ecfdf3; border-color: #abefc6; }
    .status.skipped { color: #175cd3; background: #eff8ff; border-color: #b2ddff; }
    .status.failed { color: var(--bad); background: #fef3f2; border-color: #fecdca; }
    .status.running,
    .status.mr_open,
    .status.reviewed,
    .status.pushed { color: var(--warn); background: #fffaeb; border-color: #fedf89; }

    .side {
      display: grid;
      grid-template-rows: auto minmax(0, 1fr);
      gap: 18px;
      height: min(780px, calc(100vh - 244px));
      min-height: 600px;
    }

    .inspector-panel {
      display: grid;
      grid-template-rows: auto auto;
    }

    .panel-compact .section-head {
      padding-top: 13px;
      padding-bottom: 13px;
    }

    .config-summary {
      padding: 12px 18px 16px;
      display: grid;
      gap: 10px;
    }

    .scope-picker {
      display: grid;
      gap: 6px;
      padding: 10px 12px;
      border: 1px solid var(--line-soft);
      border-radius: 8px;
      background: #fbfcfe;
      min-width: 0;
    }

    .scope-picker span {
      color: var(--faint);
      font-size: 11px;
      font-weight: 700;
      line-height: 1.2;
    }

    .scope-picker select {
      width: 100%;
      background-color: #fff;
    }

    .config-summary-grid {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 8px 14px;
    }

    .summary-item {
      min-width: 0;
    }

    .summary-label {
      color: var(--faint);
      font-size: 11px;
      font-weight: 700;
      margin-bottom: 4px;
    }

    .summary-value {
      color: var(--text);
      font-size: 13px;
      line-height: 1.35;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .config-actions {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 8px;
      align-items: stretch;
    }

    .config-actions button {
      width: 100%;
      padding: 0 10px;
    }

    .logs-panel {
      min-height: 0;
      display: grid;
      grid-template-rows: auto minmax(0, 1fr);
    }

    .logs-panel .section-head {
      align-items: center;
    }

    .kv, .logs {
      padding: 8px 18px 14px;
    }

    .logs {
      overflow: auto;
      min-height: 0;
    }

    .kv-row {
      display: grid;
      grid-template-columns: 92px minmax(0, 1fr);
      gap: 10px;
      padding: 9px 0;
      border-bottom: 1px solid var(--line-soft);
      font-size: 13px;
      line-height: 1.45;
    }

    .kv-row:last-child { border-bottom: 0; }
    .key { color: var(--muted); }
    .value { overflow-wrap: anywhere; }

    .log {
      padding: 10px 0;
      border-bottom: 1px solid var(--line-soft);
      font-size: 13px;
      line-height: 1.45;
    }

    .log:last-child { border-bottom: 0; }

    .log-step {
      display: inline-flex;
      align-items: center;
      min-height: 22px;
      border-radius: 999px;
      padding: 0 8px;
      margin: 0 7px 5px 0;
      background: #f2f4f7;
      color: #344054;
      font-weight: 700;
      font-size: 12px;
    }

    .log.error .log-step,
    .log.last-error .log-step {
      background: #fef3f2;
      color: var(--bad);
    }

    .log-time {
      color: var(--muted);
      font-size: 12px;
      margin-top: 4px;
    }

    .doctor-actions {
      display: flex;
      align-items: center;
      gap: 8px;
      flex-wrap: wrap;
      justify-content: flex-end;
    }

    .doctor-list {
      padding: 10px 18px 14px;
      overflow: auto;
    }

    .modal {
      width: min(780px, calc(100vw - 32px));
      max-height: min(760px, calc(100vh - 32px));
      padding: 0;
      border: 1px solid var(--line);
      border-radius: 8px;
      box-shadow: 0 24px 80px rgba(18, 25, 38, 0.24);
      background: #fff;
      color: var(--text);
    }

    .modal::backdrop {
      background: rgba(18, 25, 38, 0.34);
      backdrop-filter: blur(3px);
    }

    .modal-shell {
      display: grid;
      grid-template-rows: auto minmax(0, 1fr) auto;
      max-height: min(760px, calc(100vh - 32px));
    }

    .modal-head,
    .modal-foot {
      padding: 14px 18px;
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 12px;
      background: #fbfcfe;
    }

    .modal-head {
      border-bottom: 1px solid var(--line);
    }

    .modal-foot {
      border-top: 1px solid var(--line);
    }

    .modal-head h2 {
      margin: 0;
      font-size: 15px;
    }

    .modal-body {
      min-height: 0;
      overflow: auto;
    }

    .modal-status {
      color: var(--muted);
      font-size: 12px;
      line-height: 1.35;
    }

    .modal-actions {
      display: flex;
      align-items: center;
      gap: 8px;
      justify-content: flex-end;
    }

    .modal-close {
      min-width: 36px;
      padding: 0 10px;
    }

    .progress-list {
      padding: 10px 18px 14px;
    }

    .doctor-summary {
      border-radius: 8px;
      border: 1px solid var(--line);
      background: var(--surface-muted);
      padding: 10px 12px;
      margin-bottom: 8px;
      display: flex;
      justify-content: space-between;
      gap: 10px;
      color: var(--muted);
      font-size: 12px;
      line-height: 1.4;
    }

    .doctor-summary.ok {
      color: var(--good);
      border-color: #abefc6;
      background: #ecfdf3;
    }

    .doctor-summary.error {
      color: var(--bad);
      border-color: #fecdca;
      background: #fef3f2;
    }

    .doctor-check {
      display: grid;
      grid-template-columns: 28px minmax(0, 1fr);
      gap: 10px;
      padding: 10px 0;
      border-bottom: 1px solid var(--line-soft);
      font-size: 13px;
      line-height: 1.45;
    }

    .doctor-check:last-child { border-bottom: 0; }

    .doctor-mark {
      width: 24px;
      height: 24px;
      border-radius: 999px;
      display: grid;
      place-items: center;
      font-weight: 800;
      font-size: 12px;
      text-align: center;
      background: #f2f4f7;
      color: var(--muted);
    }

    .doctor-check.ok .doctor-mark { color: var(--good); background: #ecfdf3; }
    .doctor-check.warn .doctor-mark { color: var(--warn); background: #fffaeb; }
    .doctor-check.error .doctor-mark { color: var(--bad); background: #fef3f2; }
    .doctor-name { font-weight: 700; color: var(--text); }
    .doctor-fix { color: var(--muted); margin-top: 3px; overflow-wrap: anywhere; }

    .empty {
      margin: 18px;
      padding: 26px 18px;
      text-align: center;
      color: var(--muted);
      font-size: 14px;
      border: 1px dashed #cfd7e3;
      border-radius: 8px;
      background: #fbfcfe;
    }

    @media (max-width: 1080px) {
      .overview-panel {
        grid-template-columns: minmax(0, 1fr) 32px minmax(0, 1fr) 32px minmax(0, 1fr);
      }

      .overview-item.compact {
        grid-column: 1 / -1;
        border-right: 0;
        border-bottom: 1px solid var(--line-soft);
      }

      .control-panel {
        grid-template-columns: 1fr;
        align-items: stretch;
      }

      .actions { justify-content: flex-start; }
      .actions-state {
        border-left: 0;
        padding-left: 0;
      }
      .loop-options { grid-template-columns: repeat(4, minmax(0, 1fr)); }
      .queue-panel { height: min(760px, calc(100vh - 316px)); }
    }

    @media (max-width: 860px) {
      header {
        align-items: flex-start;
        flex-direction: column;
        padding: 16px 18px;
      }

      .header-meta { justify-content: flex-start; }
      main { grid-template-columns: 1fr; padding: 16px; }
      .control-title {
        display: grid;
        gap: 4px;
      }

      .control-summary { text-align: left; }
      .loop-options { grid-template-columns: repeat(2, minmax(0, 1fr)); }
      .queue-panel { height: 620px; min-height: 520px; }
      .side {
        height: auto;
        min-height: 0;
        grid-template-rows: auto 560px;
      }
      .queue-head, .queue-footer { align-items: flex-start; flex-direction: column; }
      .queue-tools { justify-content: flex-start; }
      .task { grid-template-columns: 1fr; }
      .task-state { justify-items: start; }
      .status { justify-self: start; }
    }

    @media (max-width: 560px) {
      .brand-mark { width: 36px; height: 36px; }
      .overview-panel { grid-template-columns: 1fr; }
      .overview-item,
      .overview-item:nth-child(2) {
        border-right: 0;
        border-bottom: 1px solid var(--line-soft);
      }
      .flow-arrow {
        min-height: 28px;
        border-right: 0;
        border-bottom: 1px solid var(--line-soft);
        font-size: 0;
      }
      .flow-arrow::before {
        content: "↓";
        color: var(--faint);
        font-size: 16px;
      }
      .overview-item:last-child { border-bottom: 0; }
      .loop-options { grid-template-columns: 1fr; }
      input, select, button { min-height: 44px; }
      .pager button {
        min-width: 44px;
        min-height: 44px;
      }
      .actions button { flex: 1 1 calc(50% - 8px); }
      .config-summary-grid,
      .config-actions { grid-template-columns: 1fr; }
      .meta-row { grid-template-columns: 1fr; gap: 2px; }
      .doctor-summary { flex-direction: column; }
      .modal {
        width: calc(100vw - 20px);
        max-height: calc(100vh - 20px);
      }
    }
  </style>
</head>
<body>
  <header>
    <div class="brand">
      <div class="brand-mark">MRQ</div>
      <div>
        <h1>mr-queue</h1>
        <div class="sub">跨仓 MR 队列控制台</div>
      </div>
    </div>
    <div class="header-meta">
      <div class="state-badge idle" id="runBadge"><span class="state-dot"></span><span id="subtitle">MR 队列编排器</span></div>
      <div class="header-count" id="headerCount">0 条 commit</div>
    </div>
  </header>
  <main>
    <section class="overview-panel">
      <div class="overview-item compact">
        <div class="overview-label">队列状态</div>
        <div class="overview-value" id="overviewQueue">0 条 commit</div>
        <div class="overview-sub" id="overviewProgress">暂无运行记录</div>
      </div>
      <div class="overview-item">
        <div class="overview-label">Source</div>
        <div class="overview-value" id="overviewSource">-</div>
        <div class="overview-sub" id="overviewSourceBranch">-</div>
      </div>
      <div class="flow-arrow" aria-hidden="true">›</div>
      <div class="overview-item">
        <div class="overview-label">MR Branch</div>
        <div class="overview-value" id="overviewMRBranch">-</div>
        <div class="overview-sub single-line" id="overviewMerge">-</div>
      </div>
      <div class="flow-arrow" aria-hidden="true">›</div>
      <div class="overview-item">
        <div class="overview-label">Target</div>
        <div class="overview-value" id="overviewTarget">-</div>
        <div class="overview-sub" id="overviewTargetBranch">-</div>
      </div>
    </section>
    <section class="control-panel">
      <div class="control-title">
        <h2>运行控制</h2>
        <div class="control-summary" id="controlSummary">idle</div>
      </div>
      <div class="loop-options">
        <label>检查最小<input id="waitCheckDelayMin" value="10s"></label>
        <label>检查最大<input id="waitCheckDelayMax" value="30s"></label>
        <label>下个最小<input id="nextPRDelayMin" value="1m"></label>
        <label>下个最大<input id="nextPRDelayMax" value="5m"></label>
        <label>开始时间<input id="workStart" type="time" value="08:00"></label>
        <label>结束时间<input id="workEnd" type="time" value="23:30"></label>
        <label>合入上限<input id="maxMerged" type="number" min="0" step="1" value="3"></label>
      </div>
      <div class="actions">
        <div class="actions-primary">
          <button id="syncBtn" onclick="post('/api/sync-queue')"><span class="button-icon" aria-hidden="true">↻</span>同步队列</button>
          <button class="primary" id="runBtn" onclick="post('/api/run-once')"><span class="button-icon" aria-hidden="true">▶</span>运行下一条</button>
          <button id="loopBtn" onclick="runLoop()"><span class="button-icon" aria-hidden="true">∞</span>自动运行</button>
        </div>
        <div class="actions-state">
          <button class="danger" id="stopBtn" onclick="post('/api/stop')"><span class="button-icon" aria-hidden="true">■</span>停止</button>
          <button id="pauseBtn" onclick="post('/api/pause')"><span class="button-icon" aria-hidden="true">Ⅱ</span>暂停</button>
          <button id="resumeBtn" onclick="post('/api/resume')"><span class="button-icon" aria-hidden="true">▶</span>继续</button>
        </div>
      </div>
    </section>
    <section class="queue-panel">
      <div class="section-head queue-head">
        <div class="section-title">
          <h2>Commit 队列</h2>
          <div class="section-sub" id="queueScope">按 queue_index 排序</div>
        </div>
        <div class="queue-tools">
          <div class="queue-summary" id="queueSummary"></div>
          <label class="page-size">每页
            <select id="pageSize" onchange="setPageSize(this.value)">
              <option value="10">10</option>
              <option value="20">20</option>
              <option value="50">50</option>
              <option value="100">100</option>
            </select>
          </label>
          <div class="count" id="count">0 条</div>
        </div>
      </div>
      <div class="queue" id="queue"></div>
      <div class="queue-footer">
        <div class="page-info" id="pageInfo">第 1 / 1 页</div>
        <div class="pager">
          <button id="firstPageBtn" onclick="setQueuePage(1)" aria-label="第一页">«</button>
          <button id="prevPageBtn" onclick="changeQueuePage(-1)" aria-label="上一页">‹</button>
          <button id="nextPageBtn" onclick="changeQueuePage(1)" aria-label="下一页">›</button>
          <button id="lastPageBtn" onclick="setQueuePage(queueTotalPages)" aria-label="最后一页">»</button>
        </div>
      </div>
    </section>
    <aside class="side">
      <section class="inspector-panel">
        <div class="section-head">
          <div class="section-title">
            <h2>配置概要</h2>
            <div class="section-sub" id="doctorSummary">等待检查</div>
          </div>
          <span class="count">token 已隐藏</span>
        </div>
        <div class="config-summary">
          <label class="scope-picker"><span>任务</span><select id="scopeSelect" onchange="selectScope(this.value)"><option value="">同步后生成任务</option></select></label>
          <div class="config-summary-grid" id="configSummary"></div>
          <div class="config-actions">
            <button id="doctorBtn" onclick="runDoctor(false)">检查配置</button>
            <button id="configDetailBtn" onclick="openConfigDialog()">配置详情</button>
          </div>
        </div>
      </section>
      <section class="logs-panel">
        <div class="section-head"><h2>最新日志</h2><span class="count" id="running">idle</span></div>
        <div class="logs" id="logs"></div>
      </section>
    </aside>
  </main>
  <dialog class="modal" id="doctorDialog">
    <div class="modal-shell">
      <div class="modal-head">
        <div>
          <h2 id="doctorDialogTitle">配置检查</h2>
          <div class="modal-status" id="doctorDialogStatus">等待运行</div>
        </div>
        <button class="modal-close" onclick="closeDialog('doctorDialog')" aria-label="关闭">×</button>
      </div>
      <div class="modal-body">
        <div class="progress-list" id="doctor"></div>
      </div>
      <div class="modal-foot">
        <div class="modal-status" id="doctorDialogFoot">检查结果会保存到当前服务状态。</div>
        <div class="modal-actions">
          <button class="primary" id="doctorFixBtn" onclick="runDoctor(true)" hidden>修复配置</button>
          <button onclick="closeDialog('doctorDialog')">关闭</button>
        </div>
      </div>
    </div>
  </dialog>
  <dialog class="modal" id="configDialog">
    <div class="modal-shell">
      <div class="modal-head">
        <div>
          <h2>配置详情</h2>
          <div class="modal-status">当前运行配置，敏感 token 只显示环境变量名。</div>
        </div>
        <button class="modal-close" onclick="closeDialog('configDialog')" aria-label="关闭">×</button>
      </div>
      <div class="modal-body">
        <div class="kv" id="config"></div>
      </div>
      <div class="modal-foot">
        <div class="modal-status">Source、MR Branch、Target 关系以页面顶部为准。</div>
        <div class="modal-actions">
          <button onclick="closeDialog('configDialog')">关闭</button>
        </div>
      </div>
    </div>
  </dialog>
  <script>
    const pageSizeStorageKey = 'mrQueuePageSize';
    const queuePager = {
      page: 1,
      pageSize: Number(localStorage.getItem(pageSizeStorageKey)) || 20
    };
    let queueTotalPages = 1;
    let lastStatusOK = true;
    let doctorInFlight = false;
    let scopeSwitchInFlight = false;
    let lastScopePickerSignature = '';
    let scopeSwitchError = '';

    async function post(path) {
      await fetch(path, { method: 'POST' });
      await refresh();
    }

    async function retry(sha) {
      await fetch('/api/retry?sha=' + encodeURIComponent(sha), { method: 'POST' });
      await refresh();
    }

    async function selectScope(scopeID) {
      const select = document.getElementById('scopeSelect');
      if (!scopeID || (select && select.dataset.activeScope === scopeID)) return;
      scopeSwitchInFlight = true;
      if (select) select.disabled = true;
      document.getElementById('queueScope').textContent = '正在切换任务...';
      try {
        const res = await fetch('/api/select-scope?scope_id=' + encodeURIComponent(scopeID), { method: 'POST' });
        if (!res.ok) throw new Error((await res.text()).trim() || ('HTTP ' + res.status));
        queuePager.page = 1;
        lastScopePickerSignature = '';
        scopeSwitchError = '';
        await refresh();
      } catch (err) {
        scopeSwitchError = '切换任务失败：' + (err && err.message ? err.message : err);
        await refresh();
      } finally {
        scopeSwitchInFlight = false;
      }
    }

    function openDoctorDialog(fix) {
      document.getElementById('doctorDialogTitle').textContent = fix ? '修复配置' : '配置检查';
      openDialog('doctorDialog');
    }

    function openConfigDialog() {
      openDialog('configDialog');
    }

    function openDialog(id) {
      const dialog = document.getElementById(id);
      if (dialog && !dialog.open) dialog.showModal();
    }

    function closeDialog(id) {
      const dialog = document.getElementById(id);
      if (dialog && dialog.open) dialog.close();
    }

    function setDoctorButtonsDisabled(disabled) {
      document.getElementById('doctorBtn').disabled = disabled;
      document.getElementById('doctorFixBtn').disabled = disabled;
    }

    function setDoctorFixVisible(visible) {
      document.getElementById('doctorFixBtn').hidden = !visible;
    }

    async function runLoop() {
      const params = new URLSearchParams({
        wait_check_delay_min: document.getElementById('waitCheckDelayMin').value,
        wait_check_delay_max: document.getElementById('waitCheckDelayMax').value,
        next_pr_delay_min: document.getElementById('nextPRDelayMin').value,
        next_pr_delay_max: document.getElementById('nextPRDelayMax').value,
        work_window_start: document.getElementById('workStart').value,
        work_window_end: document.getElementById('workEnd').value,
        max_merged_commits: document.getElementById('maxMerged').value
      });
      await fetch('/api/run-loop?' + params.toString(), { method: 'POST' });
      await refresh();
    }

    async function runDoctor(fix) {
      doctorInFlight = true;
      setDoctorButtonsDisabled(true);
      openDoctorDialog(fix);
      renderDoctorLoading(fix);
      try {
        const res = await fetch('/api/doctor' + (fix ? '?fix=true' : ''), { method: 'POST' });
        if (!res.ok) throw new Error('HTTP ' + res.status);
        const report = await res.json();
        renderDoctor(report);
        renderDoctorSummary(report);
        await refresh();
      } catch (err) {
        renderDoctorError(err);
      } finally {
        doctorInFlight = false;
        setDoctorButtonsDisabled(false);
      }
    }

    async function refresh() {
      let data;
      try {
        const res = await fetch('/api/status');
        data = await res.json();
        lastStatusOK = true;
      } catch (err) {
        if (lastStatusOK) handleStatusOffline();
        lastStatusOK = false;
        return;
      }
      const tasks = Object.values(data.state.tasks || {}).sort(compareTasks);
      document.getElementById('count').textContent = tasks.length + ' 条';
      document.getElementById('headerCount').textContent = tasks.length + ' 条 commit';
      document.getElementById('pageSize').value = String(queuePager.pageSize);
      document.getElementById('running').textContent = data.running ? 'running' : 'idle';
      document.getElementById('syncBtn').disabled = data.running || data.state.paused;
      document.getElementById('runBtn').disabled = data.running || data.state.paused;
      document.getElementById('loopBtn').disabled = data.running || data.state.paused;
      document.getElementById('stopBtn').disabled = !data.running;
      if (!doctorInFlight) {
        setDoctorButtonsDisabled(data.running);
      }
      for (const id of ['waitCheckDelayMin', 'waitCheckDelayMax', 'nextPRDelayMin', 'nextPRDelayMax', 'workStart', 'workEnd', 'maxMerged']) {
        document.getElementById(id).disabled = data.running;
      }
      document.getElementById('pauseBtn').disabled = data.state.paused;
      document.getElementById('resumeBtn').disabled = !data.state.paused;
      updateRunState(data);
      applyLoopDefaults(data.config || {});
      renderOverview(tasks, data.config || {});
      renderQueue(tasks);
      renderConfig(data.config || {}, data.state || {}, data.running);
      renderDoctorSummary(data.doctor);
      if (!doctorInFlight) renderDoctor(data.doctor);
      renderLogs(tasks, data.lastErr, data.lastMsg);
    }

    function handleStatusOffline() {
      const badge = document.getElementById('runBadge');
      badge.className = 'state-badge offline';
      document.getElementById('subtitle').textContent = '服务不可达';
      document.getElementById('controlSummary').textContent = '正在等待服务恢复。';
      document.getElementById('running').textContent = 'offline';
      document.getElementById('doctorSummary').textContent = '服务不可达';
      for (const id of ['syncBtn', 'runBtn', 'loopBtn', 'stopBtn', 'pauseBtn', 'resumeBtn', 'doctorBtn', 'doctorFixBtn', 'scopeSelect']) {
        document.getElementById(id).disabled = true;
      }
    }

    function setPageSize(value) {
      const parsed = Number(value);
      queuePager.pageSize = Number.isFinite(parsed) && parsed > 0 ? parsed : 20;
      queuePager.page = 1;
      localStorage.setItem(pageSizeStorageKey, String(queuePager.pageSize));
      refresh();
    }

    function setQueuePage(page) {
      queuePager.page = clampPage(page);
      refresh();
    }

    function changeQueuePage(delta) {
      setQueuePage(queuePager.page + delta);
    }

    function applyLoopDefaults(config) {
      if (window.loopDefaultsApplied) return;
      const workflow = config.workflow || {};
      document.getElementById('waitCheckDelayMin').value = workflow.wait_check_delay_min || '10s';
      document.getElementById('waitCheckDelayMax').value = workflow.wait_check_delay_max || workflow.wait_check_delay_min || '30s';
      document.getElementById('nextPRDelayMin').value = workflow.next_pr_delay_min || workflow.loop_delay_min || '1m';
      document.getElementById('nextPRDelayMax').value = workflow.next_pr_delay_max || workflow.loop_delay_max || workflow.next_pr_delay_min || '5m';
      window.loopDefaultsApplied = true;
    }

    function renderQueue(tasks) {
      const el = document.getElementById('queue');
      const footer = document.getElementById('pageInfo');
      const total = tasks.length;
      renderQueueSummary(tasks);
      queueTotalPages = Math.max(1, Math.ceil(total / queuePager.pageSize));
      queuePager.page = clampPage(queuePager.page);
      const start = (queuePager.page - 1) * queuePager.pageSize;
      const pageTasks = tasks.slice(start, start + queuePager.pageSize);
      footer.textContent = total
        ? '第 ' + queuePager.page + ' / ' + queueTotalPages + ' 页，显示 ' + (start + 1) + '-' + Math.min(start + queuePager.pageSize, total) + ' / ' + total
        : '第 1 / 1 页';
      updatePagerButtons();
      if (!tasks.length) {
        el.innerHTML = '<div class="empty">暂无队列任务，同步后会显示待处理 commit。</div>';
        return;
      }
      el.innerHTML = pageTasks.map(task => {
        const retryButton = task.status === 'failed'
          ? '<button class="retry-action" onclick="retry(' + JSON.stringify(task.sha).replaceAll('"', '&quot;') + ')">重试</button>'
          : '';
        const mr = task.mr_url
          ? '<a href="' + escapeHTML(task.mr_url) + '" target="_blank">MR #' + task.mr_number + '</a>'
          : (task.mr_number ? 'MR #' + escapeHTML(task.mr_number) : 'MR 未创建');
        const shaMap = '队列：' + escapeHTML(shortSha(task.sha)) +
          ' -> MR：' + escapeHTML(shortSha(task.mr_commit_sha) || '-') +
          ' -> 社区：' + escapeHTML(shortSha(task.community_commit_sha) || '-');
        return '<div class="task">' +
          '<div class="task-id"><div class="task-number">#' + escapeHTML(displayIndex(task)) + '</div><div class="sha">' + escapeHTML(shortSha(task.sha)) + '</div></div>' +
          '<div class="task-main"><div class="subject">' + escapeHTML(task.subject || '(no subject)') + '</div>' +
          '<div class="task-meta">' +
          '<div class="meta-row"><span class="meta-label">映射</span><span>' + shaMap + '</span></div>' +
          '<div class="meta-row"><span class="meta-label">分支</span><span>' + escapeHTML(task.branch || '-') + '</span></div>' +
          '<div class="meta-row"><span class="meta-label">MR</span><span>' + mr + '</span></div>' +
          '<div class="meta-row"><span class="meta-label">错误</span><span class="task-error">' + escapeHTML(task.error || '-') + '</span></div>' +
          '</div>' +
          '</div>' +
          '<div class="task-state"><div class="status ' + escapeHTML(task.status || '') + '">' + escapeHTML(task.status || 'pending') + '</div>' + retryButton + '</div>' +
          '</div>';
      }).join('');
    }

    function clampPage(page) {
      const parsed = Number(page);
      if (!Number.isFinite(parsed)) return 1;
      return Math.min(Math.max(1, Math.trunc(parsed)), queueTotalPages);
    }

    function updatePagerButtons() {
      document.getElementById('firstPageBtn').disabled = queuePager.page <= 1;
      document.getElementById('prevPageBtn').disabled = queuePager.page <= 1;
      document.getElementById('nextPageBtn').disabled = queuePager.page >= queueTotalPages;
      document.getElementById('lastPageBtn').disabled = queuePager.page >= queueTotalPages;
    }

    function renderQueueSummary(tasks) {
      const counts = statusCounts(tasks);
      const active = (counts.running || 0) + (counts.mr_open || 0) + (counts.reviewed || 0) + (counts.pushed || 0);
      const done = (counts.merged || 0) + (counts.skipped || 0);
      const summary = [
        ['running', active],
        ['failed', counts.failed || 0],
        ['merged', done]
      ];
      const html = summary
        .filter(([, count]) => count > 0)
        .map(([status, count]) => '<span class="status ' + escapeHTML(status) + '">' + escapeHTML(status) + ' ' + count + '</span>')
        .join('');
      const el = document.getElementById('queueSummary');
      el.innerHTML = html;
      el.hidden = html === '';
    }

    function renderOverview(tasks, config) {
      const counts = statusCounts(tasks);
      const done = (counts.merged || 0) + (counts.skipped || 0);
      const failed = counts.failed || 0;
      document.getElementById('overviewQueue').textContent = tasks.length + ' 条 commit';
      document.getElementById('overviewProgress').textContent = tasks.length
        ? '完成 ' + done + '，失败 ' + failed + '，待处理 ' + Math.max(0, tasks.length - done - failed)
        : '暂无运行记录';
      const source = config.source || {};
      const community = config.community || {};
      const queue = config.queue || {};
      const privateConfig = config.private || {};
      const sourceRepo = (source.repo || queue.remote_url || '-');
      const targetRepo = targetRepoText(config);
      setTextWithTitle('overviewSource', repoShortName(sourceRepo), compactRepo(sourceRepo));
      setTextWithTitle('overviewSourceBranch', source.branch || queue.branch || '-', source.branch || queue.branch || '-');
      setTextWithTitle('overviewTarget', repoShortName(targetRepo), compactRepo(targetRepo));
      setTextWithTitle('overviewTargetBranch', community.branch || '-', community.branch || '-');
      setTextWithTitle('overviewMRBranch', branchTemplateText(config.private || {}), mrBranchText(config));
      setTextWithTitle('overviewMerge', 'merge: ' + ((config.workflow && config.workflow.merge_method) || '-'), mrBranchText(config));
    }

    function statusCounts(tasks) {
      return tasks.reduce((acc, task) => {
        const status = String(task.status || 'pending');
        acc[status] = (acc[status] || 0) + 1;
        return acc;
      }, {});
    }

    function renderConfig(config, state, running) {
      state = state || {};
      const activeVersion = (state.config_versions || {})[state.active_config_version_id] || {};
      const activeScope = (state.task_scopes || {})[state.active_scope_id] || {};
      const taskRecordCount = Object.keys(state.task_records || {}).length;
      const resolvedRange = activeScope.resolved_commit_range || activeVersion.resolved_commit_range || (config.workflow && config.workflow.commit_range);
      renderScopePicker(state, !!running);
      document.getElementById('queueScope').textContent = scopeSwitchError || (activeScope.id
        ? '当前范围 ' + compactCommitRange(resolvedRange, activeScope, config)
        : '按队列顺序处理');
      const summaryRows = [
        ['Commit 范围', compactCommitRange(resolvedRange || commitBounds(config), activeScope, config), resolvedRange || commitBounds(config)],
        ['Commit 数', activeScope.commit_count || Object.keys(state.tasks || {}).length || '0'],
        ['任务记录', taskRecordCount || '0'],
        ['配置版本', shortID(state.active_config_version_id)]
      ];
      document.getElementById('configSummary').innerHTML = summaryRows.map(([k, v, title]) =>
        '<div class="summary-item"><div class="summary-label">' + escapeHTML(k) + '</div><div class="summary-value" title="' + escapeHTML(title || v || '-') + '">' + escapeHTML(v || '-') + '</div></div>'
      ).join('');
      const rows = [
        ['配置版本', state.active_config_version_id],
        ['任务作用域', state.active_scope_id],
        ['配置 hash', activeVersion.config_hash],
        ['作用域 commit 数', activeScope.commit_count],
        ['任务记录数', taskRecordCount || '0'],
        ['本地目录', (config.local && config.local.path) || (config.source && config.source.local_path)],
        ['requested_commit_range', activeVersion.requested_commit_range],
        ['resolved_commit_range', resolvedRange],
        ['起止 commit', commitBounds(config)],
        ['映射关系', mappingText(config)],
        ['队列分支', config.queue ? (config.queue.remote + '/' + config.queue.branch) : ''],
        ['队列基线', config.queue && config.queue.base_ref],
        ['MR 分支', config.private ? (config.private.remote + '/' + branchTemplateText(config.private)) : ''],
        ['目标仓库', config.community ? (config.community.owner + '/' + config.community.repo) : ''],
        ['目标分支', config.community && config.community.branch],
        ['合并方式', config.workflow && config.workflow.merge_method],
        ['等待评论', config.workflow && config.workflow.required_comment_text],
        ['等待检查', waitCheckDelayText(config.workflow)],
        ['下个 PR 间隔', nextPRDelayText(config.workflow)],
        ['本轮限制', '页面启动时设置'],
        ['提交账号', config.auth && config.auth.submitter && config.auth.submitter.token_env],
        ['审核账号', config.auth && config.auth.reviewer && config.auth.reviewer.token_env],
        ['合并账号', config.auth && config.auth.maintainer && config.auth.maintainer.token_env]
      ];
      document.getElementById('config').innerHTML = rows.map(([k, v]) =>
        '<div class="kv-row"><div class="key">' + escapeHTML(k) + '</div><div class="value">' + escapeHTML(v || '-') + '</div></div>'
      ).join('');
    }

    function renderScopePicker(state, running) {
      const select = document.getElementById('scopeSelect');
      if (!select) return;
      const scopes = Object.values(state.task_scopes || {}).sort(compareScopes);
      const activeScopeID = state.active_scope_id || '';
      const signature = [
        activeScopeID,
        running ? 'running' : 'idle',
        scopeSwitchInFlight ? 'switching' : 'ready',
        scopes.map(scope => [scope.id, scope.updated_at, scope.synced_at, scope.commit_count].join('/')).join('|')
      ].join('::');
      if (signature !== lastScopePickerSignature) {
        if (!scopes.length) {
          select.innerHTML = '<option value="">同步后生成任务</option>';
        } else {
          select.innerHTML = scopes.map(scope =>
            '<option value="' + escapeHTML(scope.id || '') + '">' + escapeHTML(scopeOptionLabel(scope)) + '</option>'
          ).join('');
        }
        lastScopePickerSignature = signature;
      }
      select.value = activeScopeID;
      select.dataset.activeScope = activeScopeID;
      const activeScope = scopes.find(scope => scope.id === activeScopeID);
      select.title = activeScope ? scopeFullText(activeScope) : '';
      select.disabled = running || scopeSwitchInFlight || !scopes.length;
    }

    function compareScopes(a, b) {
      return (b.synced_at || b.updated_at || '').localeCompare(a.synced_at || a.updated_at || '') ||
        (a.id || '').localeCompare(b.id || '');
    }

    function scopeOptionLabel(scope) {
      const source = repoShortName(scope.source_repo || '-');
      const target = repoShortName(scope.target_repo || '-');
      return scopeRangeText(scope) + ' · ' + source + ' -> ' + target + ' · ' + (scope.commit_count || 0) + ' commits';
    }

    function scopeBranchText(scope) {
      const source = repoShortName(scope.source_repo || '-') + ':' + (scope.source_branch || '-');
      const target = repoShortName(scope.target_repo || '-') + ':' + (scope.target_branch || '-');
      return source + ' -> ' + target;
    }

    function scopeFullText(scope) {
      const source = compactRepo(scope.source_repo || '-') + ':' + (scope.source_branch || '-');
      const target = compactRepo(scope.target_repo || '-') + ':' + (scope.target_branch || '-');
      return source + ' -> ' + target + ' · range ' + scopeFullRangeText(scope) + ' · ' + (scope.commit_count || 0) + ' commits';
    }

    function scopeRangeText(scope) {
      return compactCommitRange(scopeFullRangeText(scope), scope, {});
    }

    function scopeFullRangeText(scope) {
      return scope.resolved_commit_range || scope.requested_commit_range || '-';
    }

    function renderDoctor(report) {
      const el = document.getElementById('doctor');
      if (!report || !Array.isArray(report.checks)) {
        el.innerHTML = '<div class="empty">服务启动后会自动检查一次，也可以手动运行检查。</div>';
        setDoctorFixVisible(false);
        return;
      }
      if (!report.checks.length) {
        el.innerHTML = '<div class="empty">暂无检查结果。</div>';
        setDoctorFixVisible(false);
        return;
      }
      const counts = doctorCounts(report);
      const summaryClass = report.ok ? 'ok' : 'error';
      document.getElementById('doctorDialogStatus').textContent = report.ok ? '检查完成，配置可用。' : '检查完成，有项目需要处理。';
      document.getElementById('doctorDialogFoot').textContent = 'OK ' + (counts.ok || 0) + ' / Warn ' + (counts.warn || 0) + ' / Error ' + (counts.error || 0);
      setDoctorFixVisible(!report.ok || (counts.warn || 0) > 0 || (counts.error || 0) > 0);
      const summary = '<div class="doctor-summary ' + summaryClass + '">' +
        '<span>' + (report.ok ? '配置检查通过' : '配置需要处理') + '</span>' +
        '<span>OK ' + (counts.ok || 0) + ' / Warn ' + (counts.warn || 0) + ' / Error ' + (counts.error || 0) + '</span>' +
        '</div>';
      el.innerHTML = summary + report.checks.map(check => {
        const status = String(check.status || '');
        const mark = status === 'ok' ? '✓' : (status === 'warn' ? '!' : '✗');
        return '<div class="doctor-check ' + escapeHTML(status) + '">' +
          '<div class="doctor-mark">' + mark + '</div>' +
          '<div><div><span class="doctor-name">' + escapeHTML(check.name || '-') + '</span> ' + escapeHTML(check.message || '') + '</div>' +
          (check.fix ? '<div class="doctor-fix">fix: ' + escapeHTML(check.fix) + '</div>' : '') +
          '</div></div>';
      }).join('');
    }

    function renderDoctorSummary(report) {
      const el = document.getElementById('doctorSummary');
      if (!report || !Array.isArray(report.checks)) {
        el.textContent = '等待检查';
        return;
      }
      const counts = doctorCounts(report);
      el.textContent = report.ok
        ? '检查通过：OK ' + (counts.ok || 0)
        : '需处理：Warn ' + (counts.warn || 0) + ' / Error ' + (counts.error || 0);
    }

    function renderDoctorLoading(fix) {
      document.getElementById('doctorDialogTitle').textContent = fix ? '修复配置' : '配置检查';
      document.getElementById('doctorDialogStatus').textContent = fix ? '正在修复远端配置并检查连通性。' : '正在检查账号、远端和分支连通性。';
      document.getElementById('doctorDialogFoot').textContent = '请等待当前检查完成。';
      setDoctorFixVisible(false);
      document.getElementById('doctor').innerHTML =
        '<div class="doctor-summary"><span>' + (fix ? '修复中' : '检查中') + '</span><span>running</span></div>' +
        '<div class="doctor-check"><div class="doctor-mark">…</div><div><div><span class="doctor-name">doctor</span> 正在执行</div><div class="doctor-fix">结果返回后会在这里逐项展示。</div></div></div>';
    }

    function renderDoctorError(err) {
      document.getElementById('doctorDialogStatus').textContent = '检查请求失败';
      document.getElementById('doctorDialogFoot').textContent = '服务恢复后可以重新检查。';
      setDoctorFixVisible(false);
      document.getElementById('doctor').innerHTML =
        '<div class="doctor-summary error"><span>检查失败</span><span>request error</span></div>' +
        '<div class="doctor-check error"><div class="doctor-mark">✗</div><div><div><span class="doctor-name">api.doctor</span> ' + escapeHTML(err && err.message ? err.message : err) + '</div></div></div>';
    }

    function doctorCounts(report) {
      return (report.checks || []).reduce((acc, check) => {
        const status = String(check.status || 'error');
        acc[status] = (acc[status] || 0) + 1;
        return acc;
      }, {});
    }

    function renderLogs(tasks, lastErr, lastMsg) {
      const logs = [];
      for (const task of tasks) {
        for (const log of (task.logs || [])) {
          if (log.step === 'error' && task.status !== 'failed') continue;
          logs.push({ ...log, sha: task.sha, taskStatus: task.status, queueIndex: task.queue_index });
        }
      }
      logs.sort((a, b) => (b.time || '').localeCompare(a.time || ''));
      if (lastErr) logs.unshift({ step: 'last error', message: lastErr, time: '' });
      if (lastMsg) logs.unshift({ step: 'loop', message: lastMsg, time: '' });
      const el = document.getElementById('logs');
      if (!logs.length) {
        el.innerHTML = '<div class="empty">暂无日志</div>';
        return;
      }
      el.innerHTML = logs.slice(0, 12).map(log =>
        '<div class="log ' + escapeHTML(logClass(log)) + '"><span class="log-step">' + escapeHTML(log.step) + '</span>' + escapeHTML(displayLogMessage(log)) +
        '<div class="log-time">' + escapeHTML(formatLogTime(log.time)) + '</div></div>'
      ).join('');
    }

    function updateRunState(data) {
      const badge = document.getElementById('runBadge');
      const subtitle = document.getElementById('subtitle');
      const summary = document.getElementById('controlSummary');
      const state = data.state || {};
      let label = '空闲';
      let className = 'state-badge idle';
      if (state.paused) {
        label = '已暂停';
        className = 'state-badge paused';
      } else if (data.running) {
        label = '正在运行：' + (data.mode || 'once');
        className = 'state-badge running';
      }
      badge.className = className;
      subtitle.textContent = label;
      summary.textContent = state.paused
        ? '队列暂停中，运行入口已锁定。'
        : (data.running ? '当前任务执行中，运行参数已锁定。' : '队列未运行，运行参数可编辑。');
    }

    function formatLogTime(value) {
      if (!value) return '';
      const date = new Date(value);
      if (Number.isNaN(date.getTime())) return value;
      const parts = new Intl.DateTimeFormat('zh-CN', {
        timeZone: 'Asia/Shanghai',
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
        hour12: false
      }).formatToParts(date).reduce((acc, part) => {
        acc[part.type] = part.value;
        return acc;
      }, {});
      return parts.year + '-' + parts.month + '-' + parts.day + ' ' +
        parts.hour + ':' + parts.minute + ':' + parts.second + ' UTC+08:00';
    }

    function shortSha(sha) { return (sha || '').slice(0, 12); }
    function shortID(id) {
      const value = String(id || '');
      if (!value) return '-';
      if (value.length <= 18) return value;
      return value.slice(0, 18);
    }
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
      const target = config.community ? (targetRepoText(config) + ':' + config.community.branch) : '-';
      return base + '..' + queue + ' -> ' + mr + ' -> ' + target;
    }
    function targetRepoText(config) {
      if (config.target && config.target.repo) return config.target.repo;
      if (config.community && config.community.remote_url) return config.community.remote_url.replace(/^https?:\/\//, '').replace(/\.git$/, '');
      if (config.community && config.community.owner && config.community.repo) return config.community.owner + '/' + config.community.repo;
      return '-';
    }
    function compactRepo(value) {
      return String(value || '-').replace(/^https?:\/\//, '').replace(/\.git$/, '');
    }
    function repoShortName(value) {
      const parts = compactRepo(value).split('/').filter(Boolean);
      if (!parts.length) return '-';
      if (parts.length >= 2) return parts[parts.length - 2] + '/' + parts[parts.length - 1];
      return parts[0];
    }
    function setTextWithTitle(id, text, title) {
      const el = document.getElementById(id);
      if (!el) return;
      el.textContent = text || '-';
      el.title = title || text || '-';
    }
    function compactCommitRange(value, scope, config) {
      const text = String(value || '');
      const parts = text.split('..');
      if (parts.length === 2) {
        return refShortName(parts[0]) + '..' + refShortName(parts[1]);
      }
      if (scope && scope.source_branch && scope.target_branch) {
        return scope.target_branch + '..' + scope.source_branch;
      }
      if (config && config.source && config.source.branch && config.target && config.target.branch) {
        return config.target.branch + '..' + config.source.branch;
      }
      return value || '-';
    }
    function refShortName(value) {
      const parts = String(value || '').split('/').filter(Boolean);
      if (parts.length >= 2) return parts[parts.length - 1];
      return value || '-';
    }
    function mrBranchText(config) {
      const repo = compactRepo((config.private && config.private.remote_url) || (config.source && config.source.repo) || '');
      const branch = branchTemplateText(config.private || {});
      return repo && repo !== '-' ? repo + ':' + branch : branch;
    }
    function branchTemplateText(privateConfig) {
      const prefix = privateConfig.branch_prefix || 'mr-queue';
      return (privateConfig.branch_template || '{prefix}-{sha12}').replaceAll('{prefix}', prefix);
    }
    function waitCheckDelayText(workflow) {
      if (!workflow) return '-';
      return (workflow.wait_check_delay_min || '-') + ' .. ' + (workflow.wait_check_delay_max || workflow.wait_check_delay_min || '-');
    }
    function nextPRDelayText(workflow) {
      if (!workflow) return '-';
      const min = workflow.next_pr_delay_min || workflow.loop_delay_min || '-';
      const max = workflow.next_pr_delay_max || workflow.loop_delay_max || workflow.next_pr_delay_min || workflow.loop_delay_min || '-';
      return min + ' .. ' + max;
    }
    function displayLogMessage(log) {
      const message = String(log.message || '');
      if (log.step === 'approval' && message.includes('Approval failed but continuing')) {
        return 'Approval was rejected by the platform; continued because approval_failure_mode=warn';
      }
      return message;
    }
    function logClass(log) {
      const step = String(log.step || '').toLowerCase().replaceAll(' ', '-');
      return step === 'error' || step === 'last-error' ? step : '';
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
