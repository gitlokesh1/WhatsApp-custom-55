(() => {
  if (window.__adminUILoaded) return;
  window.__adminUILoaded = true;

  const icon = (name) => {
    const paths = {
      dashboard: '<path d="M3 13h8V3H3v10Zm0 8h8v-6H3v6Zm10 0h8V11h-8v10Zm0-18v6h8V3h-8Z"/>',
      devices: '<rect x="5" y="2" width="14" height="20" rx="3"/><path d="M9 18h6"/>',
      pair: '<path d="M8.5 14.5 6 17a3.5 3.5 0 0 1-5-5l4-4a3.5 3.5 0 0 1 5 0M15.5 9.5 18 7a3.5 3.5 0 0 1 5 5l-4 4a3.5 3.5 0 0 1-5 0M8 12h8"/>',
      health: '<path d="M12 21s8-4.5 8-11V5l-8-3-8 3v5c0 6.5 8 11 8 11Z"/><path d="m9 12 2 2 4-5"/>',
      conversations: '<path d="M21 15a4 4 0 0 1-4 4H8l-5 3V7a4 4 0 0 1 4-4h10a4 4 0 0 1 4 4v8Z"/><path d="M8 9h8M8 13h5"/>',
      bulk: '<path d="M4 5h16M4 12h16M4 19h16"/><path d="m17 2 3 3-3 3M17 9l3 3-3 3M17 16l3 3-3 3"/>',
      logs: '<path d="M6 2h9l5 5v15H6z"/><path d="M14 2v6h6M9 13h8M9 17h8"/>',
      telemetry: '<path d="M4 19V9M10 19V5M16 19v-7M22 19V2"/>',
      settings: '<circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.7 1.7 0 0 0 .34 1.88l.06.06-2.83 2.83-.06-.06A1.7 1.7 0 0 0 15 19.4a1.7 1.7 0 0 0-1 .6 1.7 1.7 0 0 0-.4 1.1V21H9.6v-.09A1.7 1.7 0 0 0 8.5 19.4a1.7 1.7 0 0 0-1.88.34l-.06.06-2.83-2.83.06-.06A1.7 1.7 0 0 0 4.6 15a1.7 1.7 0 0 0-.6-1 1.7 1.7 0 0 0-1.1-.4H3V9.6h.09A1.7 1.7 0 0 0 4.6 8.5a1.7 1.7 0 0 0-.34-1.88l-.06-.06 2.83-2.83.06.06A1.7 1.7 0 0 0 9 4.6a1.7 1.7 0 0 0 1-.6 1.7 1.7 0 0 0 .4-1.1V3h4v.09A1.7 1.7 0 0 0 15.5 4.6a1.7 1.7 0 0 0 1.88-.34l.06-.06 2.83 2.83-.06.06A1.7 1.7 0 0 0 19.4 9c.14.37.35.7.6 1 .3.3.69.4 1.1.4h.09v4h-.09A1.7 1.7 0 0 0 19.4 15Z"/>',
      schema: '<ellipse cx="12" cy="5" rx="9" ry="3"/><path d="M3 5v7c0 1.7 4 3 9 3s9-1.3 9-3V5M3 12v7c0 1.7 4 3 9 3s9-1.3 9-3v-7"/>',
      logout: '<path d="M10 17l5-5-5-5M15 12H3"/><path d="M14 3h5a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2h-5"/>',
      menu: '<path d="M4 6h16M4 12h16M4 18h16"/>',
      logo: '<path d="M20 11.4A8 8 0 0 1 8.3 18.5L3 20l1.5-5.1A8 8 0 1 1 20 11.4Z"/><path d="M9 8.5c.7 2.4 2.1 3.8 4.5 4.5"/>'
    };
    return `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${paths[name] || paths.dashboard}</svg>`;
  };

  const groups = [
    { label: 'nav.overview', items: [{ key: 'dashboard', href: '/admin', label: 'nav.dashboard', icon: 'dashboard' }] },
    { label: 'nav.whatsapp', items: [
      { key: 'devices', href: '/admin/devices', label: 'nav.devices', icon: 'devices' },
      { key: 'pair', href: '/admin/pair', label: 'nav.pair', icon: 'pair' },
      { key: 'health', href: '/admin/account-health', label: 'nav.health', icon: 'health' }
    ] },
    { label: 'nav.messaging', items: [
      { key: 'control', href: '/admin/control', label: 'nav.conversations', icon: 'conversations' },
      { key: 'bulk', href: '/admin/bulk', label: 'nav.bulk', icon: 'bulk' },
      { key: 'logs', href: '/admin/send-logs', label: 'nav.logs', icon: 'logs' },
      { key: 'telemetry', href: '/admin/telemetry', label: 'nav.telemetry', icon: 'telemetry' }
    ] },
    { label: 'nav.system', items: [
      { key: 'settings', href: '/admin/settings', label: 'nav.settings', icon: 'settings' },
      { key: 'schema', href: '/admin/schema', label: 'nav.schema', icon: 'schema' }
    ] }
  ];

  const shellMarkup = (active) => `
    <aside class="admin-sidebar" aria-label="Admin navigation">
      <a class="admin-brand" href="/admin">
        <span class="admin-brand-mark">${icon('logo')}</span>
        <span class="admin-brand-copy"><strong>88Task</strong><span data-i18n="shell.console">Operations Console</span></span>
      </a>
      ${groups.map((group,index) => `<section class="admin-nav-group"><div id="admin-nav-group-${index}" class="admin-nav-label" data-i18n="${group.label}"></div><nav class="admin-nav" aria-labelledby="admin-nav-group-${index}">${group.items.map(item => `<a href="${item.href}" class="${item.key === active ? 'active' : ''}" ${item.key === active ? 'aria-current="page"' : ''}>${icon(item.icon)}<span data-i18n="${item.label}"></span></a>`).join('')}</nav></section>`).join('')}
      <div class="admin-sidebar-footer">
        <div class="admin-live"><span data-i18n="shell.live">Service connected</span></div>
        <button class="admin-sidebar-logout" type="button" data-admin-logout>${icon('logout')}<span data-i18n="nav.logout"></span></button>
      </div>
    </aside>
    <div class="admin-backdrop" data-admin-nav-close></div>`;

  function buildShell() {
    const main = document.querySelector('main[data-admin-page]');
    if (!main || main.closest('.admin-shell')) return;
    const active = main.dataset.adminPage || 'dashboard';
    const shell = document.createElement('div');
    shell.className = 'admin-shell';
    shell.innerHTML = shellMarkup(active);
    const workspace = document.createElement('div');
    workspace.className = 'admin-workspace';
    workspace.innerHTML = `<header class="admin-topbar"><div class="admin-topbar-left"><button class="admin-menu-toggle" type="button" aria-label="Open navigation" data-admin-nav-toggle>${icon('menu')}</button><div><div class="admin-topbar-title" data-i18n="shell.console">Operations Console</div><div class="admin-topbar-context" data-i18n="shell.workspace">WhatsApp management workspace</div></div></div><div class="admin-topbar-actions"><select class="admin-language" data-admin-language aria-label="Language"></select></div></header>`;
    main.classList.add('admin-content');
    main.parentNode.insertBefore(shell, main);
    shell.appendChild(workspace);
    workspace.appendChild(main);
    shell.querySelector('[data-admin-nav-toggle]').addEventListener('click', () => shell.classList.toggle('nav-open'));
    shell.querySelector('[data-admin-nav-close]').addEventListener('click', () => shell.classList.remove('nav-open'));
    shell.querySelectorAll('.admin-nav a').forEach(link => link.addEventListener('click', () => shell.classList.remove('nav-open')));
    shell.querySelector('[data-admin-logout]').addEventListener('click', window.adminLogout);
  }

  window.adminLogout = () => {
    sessionStorage.removeItem('admin_token');
    location.href = '/admin';
  };
  window.adminHeaders = () => ({ 'X-Admin-Token': sessionStorage.getItem('admin_token') || '' });
  window.adminApi = async (url, options = {}) => {
    const headers = { ...(options.headers || {}), ...window.adminHeaders() };
    let body = options.body;
    if (body && !(body instanceof FormData)) {
      if (Object.prototype.toString.call(body) === '[object Object]') body = JSON.stringify(body);
      if (!headers['Content-Type']) headers['Content-Type'] = 'application/json';
    }
    const response = await fetch(url, { ...options, body, headers });
    if (response.status === 401) {
      window.adminLogout();
      throw new Error('Unauthorized');
    }
    const raw = await response.text();
    let data = {};
    try { data = raw ? JSON.parse(raw) : {}; } catch (_) {
      if (!response.ok) throw new Error(raw || `Request failed (${response.status})`);
      throw new Error(`Invalid server response (${response.status})`);
    }
    if (!response.ok || data.status === 'error') throw new Error(data.message || `Request failed (${response.status})`);
    return data;
  };
  window.adminEscape = (value) => String(value ?? '').replace(/[&<>"']/g, char => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[char]);
  window.adminFormatDate = (value) => {
    if (!value) return '—';
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? String(value) : date.toLocaleString();
  };
  window.adminStatusBadge = (status) => {
    const value = String(status || 'unknown').toLowerCase();
    const label = window.adminT ? window.adminT(`status.${value}`, status || 'Unknown') : (status || 'Unknown');
    return `<span class="admin-badge ${window.adminEscape(value)}"><span class="admin-dot"></span>${window.adminEscape(label)}</span>`;
  };
  window.adminSetBusy = (button, busy, label) => {
    if (!button) return;
    const count = Number(button.dataset.busyCount || 0);
    if (busy) {
      if (count === 0) button.dataset.label = button.textContent;
      button.dataset.busyCount = String(count + 1);
      button.disabled = true;
      button.textContent = label || (window.adminT ? window.adminT('common.working') : 'Working…');
    } else {
      const remaining = Math.max(0, count - 1);
      if (remaining > 0) {
        button.dataset.busyCount = String(remaining);
        return;
      }
      delete button.dataset.busyCount;
      button.disabled = false;
      if (button.dataset.label) button.textContent = button.dataset.label;
      delete button.dataset.label;
    }
  };

  function setupPopup() {
    if (document.getElementById('adminPopupRoot')) return;
    const root = document.createElement('div');
    root.id = 'adminPopupRoot';
    root.innerHTML = '<div class="ap-box" role="dialog" aria-modal="true" aria-labelledby="apTitle"><div class="ap-head" id="apTitle"></div><div class="ap-body" id="apBody"></div><div class="ap-actions" id="apActions"></div></div>';
    document.body.appendChild(root);
    const title = root.querySelector('#apTitle');
    const body = root.querySelector('#apBody');
    const actions = root.querySelector('#apActions');
    let resolver;
    const close = (result) => {
      root.classList.remove('show');
      actions.innerHTML = '';
      body.innerHTML = '';
      if (resolver) resolver(result);
      resolver = null;
    };
    const popup = (heading, message, buttons, inputOptions) => new Promise(resolve => {
      if (resolver) close({ action: 'cancel', value: null });
      resolver = resolve;
      title.textContent = heading || '88Task';
      const copy = document.createElement('div');
      copy.textContent = String(message || '');
      body.appendChild(copy);
      let input;
      if (inputOptions) {
        input = document.createElement(inputOptions.multiline ? 'textarea' : 'input');
        input.className = 'ap-input';
        input.value = inputOptions.value || '';
        input.placeholder = inputOptions.placeholder || '';
        if (!inputOptions.multiline) input.type = inputOptions.type || 'text';
        body.appendChild(input);
      }
      buttons.forEach(config => {
        const button = document.createElement('button');
        button.type = 'button';
        button.className = `ap-btn ${config.kind || ''}`;
        button.textContent = config.label;
        button.addEventListener('click', () => close({ action: config.action, value: input ? input.value : null }));
        actions.appendChild(button);
      });
      root.classList.add('show');
      setTimeout(() => (input || actions.querySelector('.primary') || actions.querySelector('button'))?.focus(), 20);
    });
    document.addEventListener('keydown', event => { if (event.key === 'Escape' && root.classList.contains('show')) close({ action: 'cancel', value: null }); });
    window.showAdminAlert = async (message, heading) => popup(heading || '88Task', message, [{ label: window.adminT ? window.adminT('common.ok') : 'OK', kind: 'primary', action: 'ok' }]);
    window.showAdminError = (message) => window.showAdminAlert(message, window.adminT ? window.adminT('common.error') : 'Error');
    window.showAdminConfirm = (message, heading, options = {}) => popup(heading || (window.adminT ? window.adminT('common.confirm') : 'Confirm action'), message, [
      { label: window.adminT ? window.adminT('common.cancel') : 'Cancel', action: 'cancel' },
      { label: options.confirmLabel || (window.adminT ? window.adminT('common.confirm') : 'Confirm'), kind: options.danger ? 'danger' : 'primary', action: 'confirm' }
    ]).then(result => result.action === 'confirm');
    window.showAdminPrompt = (message, value, heading, options = {}) => popup(heading || (window.adminT ? window.adminT('common.enter_value') : 'Enter value'), message, [
      { label: window.adminT ? window.adminT('common.cancel') : 'Cancel', action: 'cancel' },
      { label: options.confirmLabel || (window.adminT ? window.adminT('common.save') : 'Save'), kind: 'primary', action: 'ok' }
    ], { value: value || '', placeholder: options.placeholder || '', type: options.type, multiline: options.multiline }).then(result => result.action === 'ok' ? result.value : null);
  }

  window.adminToast = (message, type = '') => {
    let stack = document.querySelector('.admin-toast-stack');
    if (!stack) {
      stack = document.createElement('div');
      stack.className = 'admin-toast-stack';
      stack.setAttribute('aria-live', 'polite');
      document.body.appendChild(stack);
    }
    const toast = document.createElement('div');
    toast.className = `admin-toast ${type}`;
    toast.textContent = message;
    stack.appendChild(toast);
    setTimeout(() => toast.remove(), 4200);
  };

  function init() {
    buildShell();
    setupPopup();
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', init);
  else init();
})();
