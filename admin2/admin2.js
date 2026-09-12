// 88Task Admin 2.0 Controller
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
  const navItems = document.querySelectorAll('.nav-item');
  const tabPanes = document.querySelectorAll('.tab-pane');

  function switchTab(tabId) {
    navItems.forEach(item => {
      if (item.dataset.tab === tabId) {
        item.classList.add('active');
        item.classList.remove('text-slate-400');
        item.classList.add('text-slate-200');
      } else {
        item.classList.remove('active');
        item.classList.add('text-slate-400');
        item.classList.remove('text-slate-200');
      }
    });

    tabPanes.forEach(pane => {
      if (pane.id === `tab-${tabId}`) {
        pane.classList.remove('hidden');
      } else {
        pane.classList.add('hidden');
      }
    });

    // Close mobile drawer on tab switch
    document.getElementById('sidebar').classList.add('-left-64');
  }

  navItems.forEach(item => {
    item.addEventListener('click', (e) => {
      e.preventDefault();
      const tab = item.dataset.tab;
      window.location.hash = tab;
      switchTab(tab);
    });
  });

  // Handle Hash routing
  window.addEventListener('hashchange', () => {
    const hash = window.location.hash.replace('#', '') || 'overview';
    switchTab(hash);
  });

  // Mobile sidebar toggle
  document.getElementById('openSidebarBtn').addEventListener('click', () => {
    document.getElementById('sidebar').classList.remove('-left-64');
  });
  document.getElementById('closeSidebarBtn').addEventListener('click', () => {
    document.getElementById('sidebar').classList.add('-left-64');
  });

  // Login form handler
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
        document.getElementById('loginError').textContent = data.message || 'Invalid token';
        document.getElementById('loginError').classList.remove('hidden');
      }
    } catch (err) {
      document.getElementById('loginError').textContent = 'Connection error';
      document.getElementById('loginError').classList.remove('hidden');
    }
  });

  // Logout button
  document.getElementById('logoutBtn').addEventListener('click', () => {
    localStorage.removeItem('admin_token');
    document.cookie = 'admin_token=; Max-Age=0; path=/;';
    currentToken = '';
    showLoginModal();
  });

  // Drawer controls
  const sideDrawer = document.getElementById('sideDrawer');
  const drawerOverlay = document.getElementById('drawerOverlay');
  function openDrawer(title, htmlContent) {
    document.getElementById('drawerTitle').textContent = title;
    document.getElementById('drawerBody').innerHTML = htmlContent;
    sideDrawer.classList.remove('translate-x-full');
    drawerOverlay.classList.remove('hidden');
  }
  function closeDrawer() {
    sideDrawer.classList.add('translate-x-full');
    drawerOverlay.classList.add('hidden');
  }
  document.getElementById('closeDrawerBtn').addEventListener('click', closeDrawer);
  drawerOverlay.addEventListener('click', closeDrawer);

  // --- DATA LOADERS ---
  async function loadOverview() {
    try {
      // Load status & settings
      const [statusRes, usersRes] = await Promise.allSettled([
        apiFetch('/admin/status'),
        apiFetch('/admin/users/data?limit=1')
      ]);

      if (statusRes.status === 'fulfilled' && statusRes.value) {
        const s = statusRes.value;
        const waCount = s.active_sessions || (s.sessions ? Object.keys(s.sessions).length : 0);
        document.getElementById('metricWA').textContent = waCount;
        document.getElementById('waCountVal').textContent = waCount;
      }

      if (usersRes.status === 'fulfilled' && usersRes.value) {
        const u = usersRes.value;
        document.getElementById('metricUsers').textContent = u.total || (u.users ? u.users.length : 0);
      }

      // Load app release status for overview card
      const settingsRes = await apiFetch('/admin/settings/data');
      if (settingsRes && settingsRes.settings) {
        const sett = settingsRes.settings;
        document.getElementById('dashAppCode').textContent = sett.app_latest_version_code || '--';
        document.getElementById('dashAppName').textContent = sett.app_latest_version_name || '--';
        document.getElementById('dashForceUpdate').textContent = sett.app_force_update ? 'ENABLED' : 'DISABLED';
        document.getElementById('dashForceUpdate').className = sett.app_force_update ? 'font-bold text-rose-400' : 'text-slate-400';

        // Populate settings tab as well
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

  // Users Directory loader
  let userPage = 1;
  async function loadUsers() {
    const q = document.getElementById('userSearchInput').value.trim();
    const status = document.getElementById('userStatusFilter').value;
    const tbody = document.getElementById('usersTableBody');
    tbody.innerHTML = '<tr><td colspan="7" class="p-8 text-center text-slate-500 font-sans">Refreshing users...</td></tr>';

    try {
      const res = await apiFetch(`/admin/users/data?limit=50&page=${userPage}&search=${encodeURIComponent(q)}&status=${encodeURIComponent(status)}`);
      const users = res.users || [];
      document.getElementById('userCountSummary').textContent = `Showing ${users.length} users (Page ${userPage})`;
      
      if (users.length === 0) {
        tbody.innerHTML = '<tr><td colspan="7" class="p-8 text-center text-slate-500 font-sans">No matching users found.</td></tr>';
        return;
      }

      tbody.innerHTML = users.map(u => {
        const isSuspended = u.status === 'suspended';
        return `
          <tr class="hover:bg-slate-800/40 transition-colors">
            <td class="p-3.5">
              <div class="font-bold text-white">${u.phone || u.id}</div>
              <div class="text-[10px] text-slate-500 font-mono">${u.id}</div>
            </td>
            <td class="p-3.5">${u.country_code || '--'}</td>
            <td class="p-3.5 font-bold text-emerald-400">₹${Number(u.balance || 0).toFixed(2)}</td>
            <td class="p-3.5">
              <span class="px-2 py-0.5 rounded text-[10px] font-bold ${isSuspended ? 'bg-rose-500/10 text-rose-400 border border-rose-500/20' : 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20'}">
                ${u.status ? u.status.toUpperCase() : 'ACTIVE'}
              </span>
            </td>
            <td class="p-3.5">
              <button onclick="window.admin2ToggleWithdrawal('${u.id}', ${!u.can_withdraw})" class="text-[11px] px-2 py-1 rounded bg-slate-800 hover:bg-slate-700 ${u.can_withdraw ? 'text-emerald-400' : 'text-slate-500'}">
                ${u.can_withdraw ? 'Allowed ✓' : 'Blocked ✕'}
              </button>
            </td>
            <td class="p-3.5 text-slate-400 text-[11px]">${u.created_at ? new Date(u.created_at).toLocaleDateString() : '--'}</td>
            <td class="p-3.5 text-right space-x-2">
              <button onclick="window.admin2ToggleSuspend('${u.id}', '${isSuspended ? 'active' : 'suspended'}')" class="px-2.5 py-1 rounded-lg text-xs font-semibold ${isSuspended ? 'bg-emerald-600/20 text-emerald-400 hover:bg-emerald-600/30' : 'bg-rose-600/20 text-rose-400 hover:bg-rose-600/30'}">
                ${isSuspended ? 'Reactivate' : 'Suspend'}
              </button>
            </td>
          </tr>
        `;
      }).join('');
    } catch (e) {
      tbody.innerHTML = '<tr><td colspan="7" class="p-8 text-center text-rose-400 font-sans">Error loading users directory.</td></tr>';
    }
  }

  // Global user actions
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
      alert('Failed to update user status');
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
      alert('Failed to update withdrawal status');
    }
  };

  // Push notification send handler
  document.getElementById('sendPushBtn').addEventListener('click', async () => {
    const title = document.getElementById('pushTitle').value.trim();
    const body = document.getElementById('pushBody').value.trim();
    const imageUrl = document.getElementById('pushImageUrl').value.trim();
    if (!title || !body) {
      alert('Please provide both Title and Message body.');
      return;
    }
    const btn = document.getElementById('sendPushBtn');
    btn.disabled = true;
    btn.textContent = 'Broadcasting...';
    try {
      const res = await apiFetch('/admin/notifications/broadcast', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ title, body, image_url: imageUrl })
      });
      alert(res.message || 'Notification broadcast completed!');
      document.getElementById('pushTitle').value = '';
      document.getElementById('pushBody').value = '';
      document.getElementById('pushImageUrl').value = '';
    } catch (e) {
      alert('Broadcast error: ' + (e.message || 'Failed'));
    } finally {
      btn.disabled = false;
      btn.textContent = 'Send High-Priority Push Notification';
    }
  });

  // Live preview for push notification
  document.getElementById('pushTitle').addEventListener('input', (e) => {
    document.getElementById('previewPushTitle').textContent = e.target.value || 'Notification Title';
  });
  document.getElementById('pushBody').addEventListener('input', (e) => {
    document.getElementById('previewPushBody').textContent = e.target.value || 'Your notification message will appear here for the user.';
  });
  document.getElementById('pushImageUrl').addEventListener('input', (e) => {
    const wrap = document.getElementById('previewPushImageWrap');
    const img = document.getElementById('previewPushImg');
    if (e.target.value) {
      img.src = e.target.value;
      wrap.classList.remove('hidden');
    } else {
      wrap.classList.add('hidden');
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
      alert('App version settings updated successfully!');
      loadOverview();
    } catch (e) {
      alert('Failed to save app version settings');
    }
  });

  // Fast Pair button handler
  document.getElementById('quickPairBtn').addEventListener('click', async () => {
    const phone = document.getElementById('quickPairPhone').value.trim();
    if (!phone) {
      alert('Please enter a phone number');
      return;
    }
    const resDiv = document.getElementById('quickPairResult');
    resDiv.classList.remove('hidden');
    resDiv.textContent = 'Generating...';
    try {
      const res = await apiFetch(`/admin/pair-link?user_id=${encodeURIComponent(phone)}`, { method: 'POST' });
      if (res.status === 'success' && res.link) {
        resDiv.innerHTML = `Pairing Link Created:<br><a href="${res.link}" target="_blank" class="underline text-white">${res.link}</a><br><span class="text-slate-400 text-[10px]">Expires in 24 hours</span>`;
      } else {
        resDiv.textContent = res.message || 'Failed to create link';
      }
    } catch (e) {
      resDiv.textContent = 'Error creating pairing link';
    }
  });

  // User search inputs
  document.getElementById('userSearchInput').addEventListener('input', debounce(loadUsers, 400));
  document.getElementById('userStatusFilter').addEventListener('change', loadUsers);
  document.getElementById('userPrevPage').addEventListener('click', () => { if (userPage > 1) { userPage--; loadUsers(); } });
  document.getElementById('userNextPage').addEventListener('click', () => { userPage++; loadUsers(); });

  function debounce(func, wait) {
    let timeout;
    return function(...args) {
      clearTimeout(timeout);
      timeout = setTimeout(() => func.apply(this, args), wait);
    };
  }

  // Refresh all
  document.getElementById('refreshAllBtn').addEventListener('click', loadAllData);

  function loadAllData() {
    loadOverview();
    loadUsers();
  }

  // Initial check
  if (!currentToken) {
    showLoginModal();
  } else {
    const hash = window.location.hash.replace('#', '') || 'overview';
    switchTab(hash);
    loadAllData();
  }

})();
