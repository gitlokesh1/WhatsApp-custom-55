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
    send: '<path d="m22 2-7 20-4-9-9-4 20-7Z"/><path d="M22 2 11 13"/>'
  };

  const nav = [
    ['dashboard', '/dashboard', 'home', 'Home'], ['tasks', '/tasks', 'tasks', 'Tasks'],
    ['whatsapp', '/whatsapp', 'whatsapp', 'WhatsApp'], ['referrals', '/referrals', 'referrals', 'Referrals'],
    ['profile', '/profile', 'profile', 'Profile'], ['support', '/support', 'support', 'Support']
  ];
  const icon = (name, label = '') => `<svg class="u-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" ${label ? `role="img" aria-label="${label}"` : 'aria-hidden="true"'}>${iconPaths[name] || iconPaths.sparkle}</svg>`;
  const hydrateIcons = (root = document) => root.querySelectorAll('[data-u-icon]').forEach(node => { node.innerHTML = icon(node.dataset.uIcon); });
  const escapeHTML = value => String(value ?? '').replace(/[&<>"']/g, character => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[character]));
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
  const logout = async () => { try { await fetch('/logout', { method: 'POST' }); } finally { location.href = '/login'; } };

  function shell(page) {
    const main = document.querySelector('main[data-user-page]'); if (!main || main.closest('.u-shell')) return;
    const links = nav.map(([key, href, iconName, label]) => `<a href="${href}" class="${key === page ? 'active' : ''}" ${key === page ? 'aria-current="page"' : ''}><span class="u-nav-icon">${icon(iconName)}</span><span>${label}</span></a>`).join('');
    const shellRoot = document.createElement('div'); shellRoot.className = 'u-shell';
    shellRoot.innerHTML = `<aside class="u-sidebar"><a class="u-brand" href="/dashboard"><span class="u-brand-mark">88</span><span><strong>88Task</strong><small>Earn with every task</small></span></a><nav class="u-nav" aria-label="Primary navigation">${links}</nav><div class="u-sidebar-foot"><p>Complete verified tasks, grow your streak, and build your rewards.</p><button class="u-logout" type="button">${icon('logout')}<span>Sign out</span></button></div></aside><section class="u-workspace"><header class="u-topbar" role="banner"><div class="u-topbar-copy"><strong id="uTopGreeting">88Task workspace</strong><span>Secure WhatsApp task rewards</span></div><div class="u-wallet-pill">${icon('wallet')}<span id="uTopBalance">—</span></div></header></section><nav class="u-mobile-nav" aria-label="Mobile navigation">${links}</nav>`;
    main.parentNode.insertBefore(shellRoot, main); shellRoot.querySelector('.u-workspace').appendChild(main); shellRoot.querySelector('.u-logout').addEventListener('click', logout);
  }

  async function chooseCountry() {
    const data = await api('/api/public/countries'); const root = document.createElement('div'); root.className = 'u-modal-root show';
    root.innerHTML = `<div class="u-modal" role="dialog" aria-modal="true" aria-labelledby="countryTitle"><div class="u-eyebrow">One-time setup</div><h2 id="countryTitle">Choose your earning country</h2><p>Your country sets your currency, message reward, and daily goal. It locks after your first earning.</p><div class="u-field"><label for="countryChoice">Country</label><select class="u-select" id="countryChoice"><option value="">Select country</option>${data.countries.map(country => `<option value="${escapeHTML(country.code)}">${escapeHTML(country.name)} · ${escapeHTML(country.currency_code)} · ${money(country.reward_per_message, country.currency_code)}/message</option>`).join('')}</select></div><button class="u-btn primary block" id="saveCountry" style="margin-top:16px">Confirm country</button><p class="u-help" id="countryError" style="margin-top:10px"></p></div>`; document.body.appendChild(root);
    return new Promise(resolve => root.querySelector('#saveCountry').addEventListener('click', async () => { const code = root.querySelector('#countryChoice').value; if (!code) { root.querySelector('#countryError').textContent = 'Please choose a country.'; return; } try { await api('/api/user/country', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ country_code: code }) }); root.remove(); resolve(); } catch (error) { root.querySelector('#countryError').textContent = error.message; } }));
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
  Object.assign(window, { uEscape: escapeHTML, uMoney: money, uDate: date, uApi: api, uToast: toast, uEmpty: empty, uProfile: profile, uCelebrate: celebrate, uLogout: logout, uIcon: icon });
})();
