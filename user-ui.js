(() => {
  const iconPaths = {
    home: '<path d="M3 11.5 12 4l9 7.5"/><path d="M5.5 10v10h13V10M9.5 20v-6h5v6"/>',
    tasks: '<rect x="4" y="3" width="16" height="18" rx="3"/><path d="m8 9 1.5 1.5L12 8M8 15l1.5 1.5L12 14M14 9h2M14 15h2"/>',
    whatsapp: '<path fill="currentColor" stroke="none" d="M17.472 14.382c-.297-.149-1.758-.867-2.03-.967-.273-.099-.471-.148-.67.15-.197.297-.767.966-.94 1.164-.173.199-.347.223-.644.075-.297-.15-1.255-.463-2.39-1.475-.883-.788-1.48-1.761-1.653-2.059-.173-.297-.018-.458.13-.606.134-.133.298-.347.446-.52.149-.174.198-.298.298-.497.099-.198.05-.371-.025-.52-.075-.149-.669-1.612-.916-2.207-.242-.579-.487-.5-.669-.51-.173-.008-.371-.01-.57-.01-.198 0-.52.074-.792.372-.272.297-1.04 1.016-1.04 2.479 0 1.462 1.065 2.875 1.213 3.074.149.198 2.096 3.2 5.077 4.487.709.306 1.262.489 1.694.625.712.227 1.36.195 1.871.118.571-.085 1.758-.719 2.006-1.413.248-.694.248-1.289.173-1.413-.074-.124-.272-.198-.57-.347m-5.421 7.403h-.004a9.87 9.87 0 0 1-5.031-1.378l-.361-.214-3.741.982.998-3.648-.235-.374a9.86 9.86 0 0 1-1.51-5.26c.001-5.45 4.436-9.884 9.888-9.884 2.64 0 5.122 1.03 6.988 2.898a9.825 9.825 0 0 1 2.893 6.994c-.003 5.45-4.437 9.884-9.885 9.884m8.413-18.297A11.815 11.815 0 0 0 12.05 0C5.495 0 .16 5.335.157 11.892c0 2.096.547 4.142 1.588 5.945L.057 24l6.305-1.654a11.882 11.882 0 0 0 5.683 1.448h.005c6.554 0 11.89-5.335 11.893-11.893a11.821 11.821 0 0 0-3.48-8.413Z"/>',
    referrals: '<path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M19 8v6M16 11h6"/>',
    profile: '<circle cx="12" cy="8" r="4"/><path d="M4 21a8 8 0 0 1 16 0"/>',
    support: '<circle cx="12" cy="12" r="9"/><path d="M8.5 8.5a3.5 3.5 0 1 1 5.2 3.05c-1.1.63-1.7 1.2-1.7 2.45M12 18h.01"/>',
    logout: '<path d="m10 17 5-5-5-5M15 12H3"/><path d="M14 3h5a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2h-5"/>',
    wallet: '<path d="M4 6.5h14a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2v-13a2 2 0 0 1 2-2h12"/><path d="M16 11h6v4h-6a2 2 0 0 1 0-4Z"/>',
    flame: '<path d="M12 22c4.4 0 7-3.1 7-7.2 0-3.3-1.8-6.2-5.4-9.4.2 2.6-1.2 4.3-2.4 5.1.1-3.6-1.9-6.4-4.1-8.5.2 4.6-2.1 6.1-2.1 10.6C5 17.8 7.8 22 12 22Z"/><path d="M9.5 17.5c0-1.8 1-3 2.5-4.5.2 1.6 1.2 2.4 2.2 3.2-.2 2-1 3.8-2.4 4.7"/>',
    check: '<path d="m5 12 4 4L19 6"/>',
    activity: '<path d="M3 12h4l2.5-7 5 14 2.5-7h4"/>',
    chart: '<path d="M4 20V10M10 20V4M16 20v-7M22 20V7"/>',
    star: '<path d="m12 3 2.7 5.5 6.1.9-4.4 4.3 1 6.1-5.4-2.9-5.4 2.9 1-6.1-4.4-4.3 6.1-.9L12 3Z"/>',
    gift: '<rect x="3" y="8" width="18" height="13" rx="2"/><path d="M12 8v13M3 12h18M12 8H7.5a2.5 2.5 0 1 1 2.1-3.85L12 8Zm0 0h4.5a2.5 2.5 0 1 0-2.1-3.85L12 8Z"/>',
    sparkle: '<path d="m12 3 1.1 3.4a4 4 0 0 0 2.5 2.5L19 10l-3.4 1.1a4 4 0 0 0-2.5 2.5L12 17l-1.1-3.4a4 4 0 0 0-2.5-2.5L5 10l3.4-1.1a4 4 0 0 0 2.5-2.5L12 3ZM19 16l.5 1.5L21 18l-1.5.5L19 20l-.5-1.5L17 18l1.5-.5L19 16Z"/>',
    flag: '<path d="M5 22V4M5 5h11l-1.5 3L16 11H5"/>',
    alert: '<circle cx="12" cy="12" r="9"/><path d="M12 8v5M12 17h.01"/>',
    lock: '<rect x="5" y="10" width="14" height="11" rx="2"/><path d="M8 10V7a4 4 0 0 1 8 0v3"/>',
    globe: '<circle cx="12" cy="12" r="9"/><path d="M3 12h18M12 3a15 15 0 0 1 0 18M12 3a15 15 0 0 0 0 18"/>',
    medal: '<circle cx="12" cy="9" r="5"/><path d="m8.5 13-2 8 5.5-3 5.5 3-2-8"/>',
    message: '<path d="M21 15a4 4 0 0 1-4 4H8l-5 3V7a4 4 0 0 1 4-4h10a4 4 0 0 1 4 4v8Z"/><path d="M8 9h8M8 13h5"/>',
    arrowRight: '<path d="M5 12h14M14 7l5 5-5 5"/>',
    refresh: '<path d="M20 6v5h-5M4 18v-5h5"/><path d="M18.5 10a7 7 0 0 0-12-3L4 11M5.5 14a7 7 0 0 0 12 3l2.5-4"/>',
    share: '<circle cx="18" cy="5" r="3"/><circle cx="6" cy="12" r="3"/><circle cx="18" cy="19" r="3"/><path d="m8.6 10.5 6.8-4M8.6 13.5l6.8 4"/>',
    copy: '<rect x="8" y="8" width="12" height="12" rx="2"/><path d="M16 8V6a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h2"/>',
    phone: '<rect x="7" y="2" width="10" height="20" rx="3"/><path d="M10 18h4"/>',
    send: '<path d="m22 2-7 20-4-9-9-4 20-7Z"/><path d="M22 2 11 13"/>',
    close: '<path d="m6 6 12 12M18 6 6 18"/>',
    chevronDown: '<path d="m6 9 6 6 6-6"/>',
    announcement: '<path d="M3 11v2a2 2 0 0 0 2 2h2l2 5h3l-2-5 9-4V5L7 9H5a2 2 0 0 0-2 2Z"/><path d="M19 9c1.3.7 2 1.7 2 3s-.7 2.3-2 3"/>',
    search: '<circle cx="11" cy="11" r="7"/><path d="m20 20-4-4"/>'
  };

  const nav = [
    ['dashboard', '/dashboard', 'home', 'Home'], ['tasks', '/tasks', 'tasks', 'Tasks'],
    ['whatsapp', '/whatsapp', 'whatsapp', 'WhatsApp'], ['referrals', '/referrals', 'referrals', 'Referrals'],
    ['profile', '/profile', 'profile', 'Profile'], ['support', '/support', 'support', 'Support']
  ];
  const escapeHTML = value => String(value ?? '').replace(/[&<>"']/g, character => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[character]));
  const icon = (name, label = '') => `<svg class="u-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" ${label ? `role="img" aria-label="${escapeHTML(label)}"` : 'aria-hidden="true"'}>${iconPaths[name] || iconPaths.sparkle}</svg>`;
  const hydrateIcons = (root = document) => root.querySelectorAll('[data-u-icon]').forEach(node => { node.innerHTML = icon(node.dataset.uIcon); });
  const safeInternalURL = value => typeof value === 'string' && value.startsWith('/') && !value.startsWith('//') ? value : '';
  const money = (amount, currency = 'INR') => { try { return new Intl.NumberFormat(undefined, { style: 'currency', currency: currency || 'INR', maximumFractionDigits: 2 }).format(Number(amount) || 0); } catch (_) { return `${currency || ''} ${(Number(amount) || 0).toFixed(2)}`; } };
  const date = value => value ? new Date(value).toLocaleString() : '—';
  const api = async (url, options = {}) => {
    const response = await fetch(url, options); const data = await response.json().catch(() => ({}));
    if (response.status === 401) { location.href = '/login'; throw new Error('Login required'); }
    if (!response.ok || data.status === 'error') throw new Error(data.message || 'Something went wrong'); return data;
  };
  const toast = (message, type = '') => {
    let stack = document.querySelector('.u-toast-stack');
    if (!stack) { stack = document.createElement('div'); stack.className = 'u-toast-stack'; stack.setAttribute('aria-live', 'polite'); document.body.appendChild(stack); }
    const item = document.createElement('div'); item.className = `u-toast ${type}`; item.textContent = message; stack.appendChild(item); setTimeout(() => item.remove(), 4200);
  };
  const empty = (iconName, title, copy = '') => `<div class="u-empty"><div class="u-empty-icon">${icon(iconName)}</div><strong>${escapeHTML(title)}</strong>${copy ? `<div>${escapeHTML(copy)}</div>` : ''}</div>`;
  const countryFlag = (code, name = '') => {
    const normalized = String(code || '').toLowerCase();
    return /^[a-z]{2}$/.test(normalized) ? `<img src="https://flagcdn.com/w80/${normalized}.png" alt="" loading="lazy" onerror="this.hidden=true">` : icon('globe', name ? `${name} country` : 'Country');
  };
  const mountModal = (markup, options = {}) => {
    const previousFocus = document.activeElement;
    const root = document.createElement('div'); root.className = 'u-modal-root show'; root.innerHTML = markup; document.body.appendChild(root); document.body.classList.add('u-modal-open');
    const close = value => { root.remove(); document.removeEventListener('keydown', onKeyDown); if (!document.querySelector('.u-modal-root.show')) document.body.classList.remove('u-modal-open'); if (previousFocus?.focus) previousFocus.focus(); options.onClose?.(value); };
    const onKeyDown = event => { if (event.key === 'Escape' && options.dismissible !== false) close(options.cancelValue); if (event.key === 'Tab') { const focusable = [...root.querySelectorAll('button:not([disabled]),input:not([disabled]),select:not([disabled]),textarea:not([disabled]),a[href]')].filter(node => !node.hidden); if (!focusable.length) return; const first = focusable[0], last = focusable[focusable.length - 1]; if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); } else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); } } };
    document.addEventListener('keydown', onKeyDown);
    if (options.dismissible !== false) root.addEventListener('mousedown', event => { if (event.target === root) close(options.cancelValue); });
    requestAnimationFrame(() => root.querySelector('[autofocus],button,input,select,textarea,a[href]')?.focus());
    return { root, close };
  };
  const confirmDialog = (title, message, options = {}) => new Promise(resolve => {
    const actions = `<button class="u-btn" type="button" data-u-cancel>${escapeHTML(options.cancelText || 'Cancel')}</button><button class="u-btn ${options.danger ? 'danger' : 'primary'}" type="button" data-u-confirm>${escapeHTML(options.confirmText || 'Confirm')}</button>`;
    const modal = mountModal(`<div class="u-modal u-confirm-modal" role="alertdialog" aria-modal="true" aria-labelledby="confirmTitle" aria-describedby="confirmCopy"><button class="u-modal-close" type="button" aria-label="Close">${icon('close')}</button><div class="u-dialog-icon ${options.danger ? 'danger' : ''}">${icon(options.icon || (options.danger ? 'alert' : 'check'))}</div><h2 id="confirmTitle">${escapeHTML(title)}</h2><p id="confirmCopy">${escapeHTML(message)}</p><div class="u-dialog-actions">${actions}</div></div>`, { cancelValue: false, onClose: resolve });
    modal.root.querySelector('[data-u-cancel]').onclick = () => modal.close(false);
    modal.root.querySelector('.u-modal-close').onclick = () => modal.close(false);
    modal.root.querySelector('[data-u-confirm]').onclick = () => modal.close(true);
  });
  const countryPicker = (countries, selectedCode = '', options = {}) => new Promise(resolve => {
    const normalized = Array.isArray(countries) ? countries : [];
    const closeButton = options.required ? '' : `<button class="u-modal-close" type="button" aria-label="Close">${icon('close')}</button>`;
    const rows = normalized.map(country => `<button class="u-country-option ${country.code === selectedCode ? 'selected' : ''}" type="button" role="option" aria-selected="${country.code === selectedCode}" data-country-code="${escapeHTML(country.code)}"><span class="u-country-flag" role="img" aria-label="${escapeHTML(country.name)} flag">${countryFlag(country.code, country.name)}</span><span class="u-country-option-copy"><strong>${escapeHTML(country.name)}</strong><small>${escapeHTML(country.currency_code)} · ${money(country.reward_per_message, country.currency_code)} per message · Goal ${country.daily_goal}/day</small></span><span class="u-country-check">${icon('check')}</span></button>`).join('');
    const modal = mountModal(`<div class="u-modal u-country-modal" role="dialog" aria-modal="true" aria-labelledby="countryPickerTitle">${closeButton}<div class="u-dialog-icon">${icon('globe')}</div><div class="u-eyebrow">Local earning setup</div><h2 id="countryPickerTitle">Choose your earning country</h2><p>Your country sets your reward currency, rate, and daily target.</p><div class="u-country-search"><span>${icon('search')}</span><input type="search" autocomplete="off" placeholder="Search country or currency" aria-label="Search countries" autofocus></div><div class="u-country-options" role="listbox" aria-label="Available earning countries">${rows || '<div class="u-empty">No active countries are available.</div>'}</div><div class="u-country-no-results" hidden>No matching countries found.</div></div>`, { dismissible: !options.required, cancelValue: null, onClose: resolve });
    const search = modal.root.querySelector('input[type="search"]');
    const optionsList = [...modal.root.querySelectorAll('[data-country-code]')];
    if (search) search.oninput = () => { const query = search.value.trim().toLowerCase(); let visible = 0; optionsList.forEach(button => { const matches = button.textContent.toLowerCase().includes(query); button.hidden = !matches; if (matches) visible++; }); modal.root.querySelector('.u-country-no-results').hidden = visible !== 0; };
    optionsList.forEach(button => button.onclick = () => modal.close(normalized.find(country => country.code === button.dataset.countryCode) || null));
    modal.root.querySelector('.u-modal-close')?.addEventListener('click', () => modal.close(null));
  });
  const eventSeenKey = '88task_seen_events';
  const readSeenEvents = () => {
    try { const value = JSON.parse(sessionStorage.getItem(eventSeenKey) || '[]'); return new Set(Array.isArray(value) ? value.map(String) : []); } catch (_) { return new Set(); }
  };
  const clearSeenEvents = () => { sessionStorage.removeItem(eventSeenKey); sessionStorage.removeItem('88task_seen_event'); };
  const showEvent = campaigns => {
    if (document.querySelector('.u-event-modal')) return false;
    const seen = readSeenEvents();
    const queue = (Array.isArray(campaigns) ? campaigns : [campaigns]).filter(banner => banner?.show_as_popup && !seen.has(String(banner.id)));
    if (!queue.length) return false;
    const showNext = () => {
      const banner = queue.shift(); if (!banner) return;
      seen.add(String(banner.id)); sessionStorage.setItem(eventSeenKey, JSON.stringify([...seen]));
      const artwork = banner.image_url ? `<img class="u-event-art" src="${escapeHTML(banner.image_url)}" alt="${escapeHTML(banner.alt_text || '')}">` : `<div class="u-event-art u-event-fallback">${icon('announcement')}</div>`;
      const ctaURL = safeInternalURL(banner.cta_url); const cta = banner.cta_label && ctaURL ? `<a class="u-btn primary" href="${escapeHTML(ctaURL)}">${escapeHTML(banner.cta_label)} ${icon('arrowRight')}</a>` : '';
      const modal = mountModal(`<div class="u-modal u-event-modal" role="dialog" aria-modal="true" aria-labelledby="eventTitle"><button class="u-modal-close" type="button" aria-label="Dismiss announcement">${icon('close')}</button>${artwork}<div class="u-event-copy"><div class="u-eyebrow">Latest from 88Task</div><h2 id="eventTitle">${escapeHTML(banner.title)}</h2><p>${escapeHTML(banner.body || '')}</p><div class="u-dialog-actions">${cta}<button class="u-btn" type="button" data-u-dismiss>Maybe later</button></div></div></div>`, { cancelValue: null, onClose: showNext });
      modal.root.querySelector('.u-modal-close').onclick = () => modal.close(); modal.root.querySelector('[data-u-dismiss]').onclick = () => modal.close();
    };
    showNext(); return true;
  };
  const logout = async () => { clearSeenEvents(); try { await fetch('/logout', { method: 'POST' }); } finally { location.href = '/login'; } };

  function shell(page) {
    const main = document.querySelector('main[data-user-page]'); if (!main || main.closest('.u-shell')) return;
    const links = nav.map(([key, href, iconName, label]) => `<a href="${href}" class="${key === page ? 'active' : ''}" ${key === page ? 'aria-current="page"' : ''}><span class="u-nav-icon">${icon(iconName)}</span><span>${label}</span></a>`).join('');
    const shellRoot = document.createElement('div'); shellRoot.className = 'u-shell';
    shellRoot.innerHTML = `<aside class="u-sidebar"><a class="u-brand" href="/dashboard"><span class="u-brand-mark">88</span><span><strong>88Task</strong><small>Earn with every task</small></span></a><nav class="u-nav" aria-label="Primary navigation">${links}</nav><div class="u-sidebar-foot"><p>Complete verified tasks, grow your streak, and build your rewards.</p><button class="u-logout" type="button">${icon('logout')}<span>Sign out</span></button></div></aside><section class="u-workspace"><header class="u-topbar" role="banner"><div class="u-topbar-copy"><strong id="uTopGreeting">88Task workspace</strong><span>Secure WhatsApp task rewards</span></div><div class="u-wallet-pill">${icon('wallet')}<span id="uTopBalance">—</span></div></header></section><nav class="u-mobile-nav" aria-label="Mobile navigation">${links}</nav>`;
    main.parentNode.insertBefore(shellRoot, main); shellRoot.querySelector('.u-workspace').appendChild(main); shellRoot.querySelector('.u-logout').addEventListener('click', logout);
    const footer = document.createElement('footer'); footer.className = 'u-user-footer'; footer.innerHTML = `<span><strong>88Task</strong> · Verified task rewards</span><button class="u-btn u-footer-logout" type="button">${icon('logout')}Sign out</button>`; main.appendChild(footer); footer.querySelector('button').addEventListener('click', logout);
  }

  async function chooseCountry() {
    const data = await api('/api/public/countries'); if (!data.countries?.length) throw new Error('No earning countries are currently available'); const country = await countryPicker(data.countries, '', { required: true });
    await api('/api/user/country', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ country_code: country.code }) });
  }
  async function profile(requireCountry = true) {
    let data = await api('/api/user/profile'); if (data.requires_country && requireCountry) { await chooseCountry(); data = await api('/api/user/profile'); }
    const greeting = document.getElementById('uTopGreeting'); if (greeting) greeting.textContent = `Hello, ${data.name}`;
    const balance = document.getElementById('uTopBalance'); if (balance) balance.textContent = money(data.balance, data.currency_code); window.userProfile = data; return data;
  }
  function celebrate(reward, currency, gamification) {
    const root = document.createElement('div'); root.className = 'u-modal-root show'; root.innerHTML = `<div class="u-modal u-celebrate" role="dialog" aria-modal="true"><div class="u-celebrate-icon">${icon('sparkle')}</div><div class="u-eyebrow">Reward unlocked</div><h2>${money(reward, currency)} earned!</h2><p>Your verified message was sent successfully. You are now on a ${gamification?.streak_days || 0}-day streak.</p><button class="u-btn primary" type="button">Keep earning</button></div>`; document.body.appendChild(root); root.querySelector('button').onclick = () => root.remove(); root.querySelector('button').focus();
  }
  document.addEventListener('DOMContentLoaded', () => { hydrateIcons(); const main = document.querySelector('main[data-user-page]'); if (main) shell(main.dataset.userPage); });
  Object.assign(window, { uEscape: escapeHTML, uMoney: money, uDate: date, uApi: api, uToast: toast, uEmpty: empty, uProfile: profile, uCelebrate: celebrate, uLogout: logout, uIcon: icon, uConfirm: confirmDialog, uCountryPicker: countryPicker, uCountryFlag: countryFlag, uShowEvent: showEvent, uSafeURL: safeInternalURL });
})();
