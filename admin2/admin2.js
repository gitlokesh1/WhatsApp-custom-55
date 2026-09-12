// 88Task Admin 2.0 Pure White Controller
(function() {
  'use strict';

  let currentToken = localStorage.getItem('admin_token') || getCookie('admin_token') || '';
  const urlParams = new URLSearchParams(window.location.search);
  const qToken = urlParams.get('token');
  if (qToken) {
    currentToken = qToken;
    localStorage.setItem('admin_token', qToken);
  }

  function getCookie(name) {
    const value = `; ${document.cookie}`;
    const parts = value.split(`; ${name}=`);
    if (parts.length === 2) return parts.pop().split(';').shift();
    return '';
  }

  async function apiFetch(url, options = {}) {
    options.headers = options.headers || {};
    if (currentToken) {
      options.headers['X-Admin-Token'] = currentToken;
    }
    const res = await fetch(url, options);
    if (res.status === 401) {
      showLoginModal();
      throw new Error('Unauthorized');
    }
    return res.json();
  }

  function showLoginModal() {
    document.getElementById('loginModal').classList.remove('hidden');
  }

  function hideLoginModal() {
    document.getElementById('loginModal').classList.add('hidden');
  }

  // Handle Tab Switching
  const navLinks = document.querySelectorAll('.nav-link');
  const tabPanes = document.querySelectorAll('.tab-pane');

  function switchTab(tabId) {
    navLinks.forEach(link => {
      if (link.dataset.tab === tabId) {
        link.classList.add('active');
      } else {
        link.classList.remove('active');
      }
    });

    tabPanes.forEach(pane => {
      if (pane.id === `tab-${tabId}`) {
        pane.classList.remove('hidden');
      } else {
        pane.classList.add('hidden');
      }
    });
  }

  navLinks.forEach(link => {
    link.addEventListener('click', (e) => {
      e.preventDefault();
      const tab = link.dataset.tab;
      window.location.hash = tab;
      switchTab(tab);
    });
  });

  window.addEventListener('hashchange', () => {
    const hash = window.location.hash.replace('#', '') || 'overview';
    switchTab(hash);
  });

  // Login handler
  document.getElementById('loginForm').addEventListener('submit', async (e) => {
    e.preventDefault();
    const token = document.getElementById('loginTokenInput').value.trim();
    try {
      const res = await fetch('/admin2/api/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ token })
      });
      const data = await res.json();
      if (data.status === 'success') {
        currentToken = token;
        localStorage.setItem('admin_token', token);
        hideLoginModal();
        loadAllData();
      } else {
        const errDiv = document.getElementById('loginError');
        errDiv.textContent = data.message || 'Invalid token';
        errDiv.classList.remove('hidden');
      }
    } catch (err) {
      const errDiv = document.getElementById('loginError');
      errDiv.textContent = 'Connection error';
      errDiv.classList.remove('hidden');
    }
  });

  // Logout handler
  document.getElementById('logoutBtn').addEventListener('click', () => {
    localStorage.removeItem('admin_token');
    document.cookie = 'admin_token=; Max-Age=0; path=/;';
    currentToken = '';
    showLoginModal();
  });

  // Overview loader
  async function loadOverview() {
    try {
      const [statusRes, usersRes] = await Promise.allSettled([
        apiFetch('/admin/status'),
        apiFetch('/admin/users/data?limit=1')
      ]);

      if (statusRes.status === 'fulfilled' && statusRes.value) {
        const s = statusRes.value;
        const waCount = s.active_sessions || (s.sessions ? Object.keys(s.sessions).length : 0);
        document.getElementById('metricWA').textContent = waCount;
        document.getElementById('waHeaderBadge').textContent = `WA: ${waCount} Online`;
      }

      if (usersRes.status === 'fulfilled' && usersRes.value) {
        const u = usersRes.value;
        document.getElementById('metricUsers').textContent = u.total || (u.users ? u.users.length : 0);
      }

      const settingsRes = await apiFetch('/admin/settings/data');
      if (settingsRes && settingsRes.settings) {
        const sett = settingsRes.settings;
        document.getElementById('dashAppCode').textContent = sett.app_latest_version_code || '--';
        document.getElementById('dashAppName').textContent = sett.app_latest_version_name || '--';
        document.getElementById('dashForceUpdate').textContent = sett.app_force_update ? 'ENABLED' : 'DISABLED';

        document.getElementById('appVerCode').value = sett.app_latest_version_code || '';
        document.getElementById('appVerName').value = sett.app_latest_version_name || '';
        document.getElementById('appDownloadUrl').value = sett.app_download_url || '';
        document.getElementById('appUpdateMsg').value = sett.app_update_message || '';
        document.getElementById('appForceUpdate').checked = !!sett.app_force_update;

        document.getElementById('settWarmupEnabled').checked = !!sett.warmup_enabled;
        document.getElementById('settWarmup1').value = sett.warmup_day1_limit || 5;
        document.getElementById('settWarmup2').value = sett.warmup_day2_limit || 10;
        document.getElementById('settWarmup3').value = sett.warmup_day3_limit || 15;
        document.getElementById('settMaxHour').value = sett.safety_max_messages_hour || 20;
        document.getElementById('settMaxDay').value = sett.safety_max_messages_day || 100;
      }
    } catch (e) {
      console.warn('Overview fetch error:', e);
    }
  }

  // Users loader
  let userPage = 1;
  async function loadUsers() {
    const q = document.getElementById('userSearchInput').value.trim();
    const status = document.getElementById('userStatusFilter').value;
    const tbody = document.getElementById('usersTableBody');
    tbody.innerHTML = '<tr><td colspan="7" style="text-align: center; padding: 30px; color: #64748b;">Loading users...</td></tr>';

    try {
      const res = await apiFetch(`/admin/users/data?limit=50&page=${userPage}&search=${encodeURIComponent(q)}&status=${encodeURIComponent(status)}`);
      const users = res.users || [];
      document.getElementById('userCountSummary').textContent = `Showing ${users.length} users (Page ${userPage})`;
      
      if (users.length === 0) {
        tbody.innerHTML = '<tr><td colspan="7" style="text-align: center; padding: 30px; color: #64748b;">No users found.</td></tr>';
        return;
      }

      tbody.innerHTML = users.map(u => {
        const isSuspended = u.status === 'suspended';
        return `
          <tr>
            <td>
              <strong style="color: #0f172a; font-size: 14px;">${u.phone || u.id}</strong>
              <div style="font-size: 10px; color: #64748b;" class="font-mono">${u.id}</div>
            </td>
            <td><strong>${u.country_code || '--'}</strong></td>
            <td><strong style="color: #059669; font-size: 14px;">₹${Number(u.balance || 0).toFixed(2)}</strong></td>
            <td>
              <span class="badge ${isSuspended ? 'badge-suspended' : 'badge-active'}">
                ${u.status ? u.status.toUpperCase() : 'ACTIVE'}
              </span>
            </td>
            <td>
              <button onclick="window.admin2ToggleWithdrawal('${u.id}', ${!u.can_withdraw})" class="btn btn-outline btn-sm">
                ${u.can_withdraw ? 'Allowed ✓' : 'Blocked ✕'}
              </button>
            </td>
            <td style="color: #64748b; font-size: 12px;">${u.created_at ? new Date(u.created_at).toLocaleDateString() : '--'}</td>
            <td class="text-right">
              <button onclick="window.admin2ToggleSuspend('${u.id}', '${isSuspended ? 'active' : 'suspended'}')" class="btn ${isSuspended ? 'btn-emerald' : 'btn-danger-outline'} btn-sm">
                ${isSuspended ? 'Reactivate' : 'Suspend'}
              </button>
            </td>
          </tr>
        `;
      }).join('');
    } catch (e) {
      tbody.innerHTML = '<tr><td colspan="7" style="text-align: center; padding: 30px; color: #b91c1c;">Error loading user directory.</td></tr>';
    }
  }

  window.admin2ToggleSuspend = async function(userId, newStatus) {
    if (!confirm(`Are you sure you want to change user status to ${newStatus}?`)) return;
    try {
      await apiFetch('/admin/users/status', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ user_id: userId, status: newStatus })
      });
      loadUsers();
    } catch (e) {
      alert('Failed to update status');
    }
  };

  window.admin2ToggleWithdrawal = async function(userId, allow) {
    try {
      await apiFetch('/admin/users/withdrawal-status', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ user_id: userId, can_withdraw: allow })
      });
      loadUsers();
    } catch (e) {
      alert('Failed to update withdrawal permission');
    }
  };

  // Push notification handlers
  document.getElementById('sendPushBtn').addEventListener('click', async () => {
    const title = document.getElementById('pushTitle').value.trim();
    const body = document.getElementById('pushBody').value.trim();
    const imageUrl = document.getElementById('pushImageUrl').value.trim();
    if (!title || !body) {
      alert('Please provide Title and Body');
      return;
    }
    try {
      const res = await apiFetch('/admin/notifications/broadcast', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ title, body, image_url: imageUrl })
      });
      alert(res.message || 'Notification broadcast completed!');
    } catch (e) {
      alert('Broadcast error: ' + (e.message || 'Failed'));
    }
  });

  // App Release save handler
  document.getElementById('saveAppReleaseBtn').addEventListener('click', async () => {
    const code = document.getElementById('appVerCode').value.trim();
    const name = document.getElementById('appVerName').value.trim();
    const url = document.getElementById('appDownloadUrl').value.trim();
    const msg = document.getElementById('appUpdateMsg').value.trim();
    const force = document.getElementById('appForceUpdate').checked;

    try {
      await apiFetch('/admin/settings/data', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          app_latest_version_code: code,
          app_latest_version_name: name,
          app_download_url: url,
          app_update_message: msg,
          app_force_update: force
        })
      });
      alert('App version settings saved!');
      loadOverview();
    } catch (e) {
      alert('Failed to save version settings');
    }
  });

  // Fast pair button
  document.getElementById('quickPairBtn').addEventListener('click', async () => {
    const phone = document.getElementById('quickPairPhone').value.trim();
    if (!phone) {
      alert('Please enter phone number');
      return;
    }
    const resDiv = document.getElementById('quickPairResult');
    resDiv.classList.remove('hidden');
    resDiv.textContent = 'Generating pairing link...';
    try {
      const res = await apiFetch(`/admin/pair-link?user_id=${encodeURIComponent(phone)}`, { method: 'POST' });
      if (res.status === 'success' && res.link) {
        resDiv.innerHTML = `<strong>Pairing Link Created:</strong><br><a href="${res.link}" target="_blank" style="color: #2563eb; font-weight: 700;">${res.link}</a>`;
      } else {
        resDiv.textContent = res.message || 'Failed';
      }
    } catch (e) {
      resDiv.textContent = 'Error creating link';
    }
  });

  // Search listeners
  document.getElementById('userSearchInput').addEventListener('input', debounce(loadUsers, 400));
  document.getElementById('userStatusFilter').addEventListener('change', loadUsers);
  document.getElementById('userPrevPage').addEventListener('click', () => { if (userPage > 1) { userPage--; loadUsers(); } });
  document.getElementById('userNextPage').addEventListener('click', () => { userPage++; loadUsers(); });
  document.getElementById('refreshAllBtn').addEventListener('click', loadAllData);

  function debounce(func, wait) {
    let timeout;
    return function(...args) {
      clearTimeout(timeout);
      timeout = setTimeout(() => func.apply(this, args), wait);
    };
  }

  function loadAllData() {
    loadOverview();
    loadUsers();
  }

  if (!currentToken) {
    showLoginModal();
  } else {
    const hash = window.location.hash.replace('#', '') || 'overview';
    switchTab(hash);
    loadAllData();
  }

})();
