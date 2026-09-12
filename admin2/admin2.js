// 88Task Admin 2.0 Fully Wired Pure White Controller
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
    try {
      const res = await fetch(url, options);
      if (res.status === 401) {
        showLoginModal();
        throw new Error('Unauthorized');
      }
      return await res.json();
    } catch (e) {
      console.error('API call failed for ' + url, e);
      throw e;
    }
  }

  function showLoginModal() {
    const modal = document.getElementById('loginModal');
    if (modal) modal.classList.remove('hidden');
  }

  function hideLoginModal() {
    const modal = document.getElementById('loginModal');
    if (modal) modal.classList.add('hidden');
  }

  // Tab Navigation
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

    // Trigger data loader for active tab
    if (tabId === 'overview') loadOverview();
    else if (tabId === 'users') loadUsers();
    else if (tabId === 'campaigns') loadCampaigns();
    else if (tabId === 'whatsapp') loadWhatsApp();
    else if (tabId === 'withdrawals') loadWithdrawals();
    else if (tabId === 'broadcasts') loadBroadcasts();
    else if (tabId === 'apprelease' || tabId === 'settings') loadSettingsData();
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
  const loginForm = document.getElementById('loginForm');
  if (loginForm) {
    loginForm.addEventListener('submit', async (e) => {
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
          if (errDiv) {
            errDiv.textContent = data.message || 'Invalid token';
            errDiv.classList.remove('hidden');
          }
        }
      } catch (err) {
        const errDiv = document.getElementById('loginError');
        if (errDiv) {
          errDiv.textContent = 'Connection error';
          errDiv.classList.remove('hidden');
        }
      }
    });
  }

  // Logout handler
  const logoutBtn = document.getElementById('logoutBtn');
  if (logoutBtn) {
    logoutBtn.addEventListener('click', () => {
      localStorage.removeItem('admin_token');
      document.cookie = 'admin_token=; Max-Age=0; path=/;';
      currentToken = '';
      showLoginModal();
    });
  }

  // ==================== OVERVIEW ====================
  async function loadOverview() {
    try {
      const [statusRes, usersRes, campRes, withRes, settRes] = await Promise.allSettled([
        apiFetch('/admin/status'),
        apiFetch('/admin/users/data?limit=1'),
        apiFetch('/admin/campaigns/data'),
        apiFetch('/admin/withdrawals/data'),
        apiFetch('/admin/settings/data')
      ]);

      if (statusRes.status === 'fulfilled' && statusRes.value) {
        const s = statusRes.value;
        const waCount = s.active_sessions || (s.sessions ? Object.keys(s.sessions).length : 0);
        document.getElementById('metricWA').textContent = waCount;
        const badge = document.getElementById('waHeaderBadge');
        if (badge) badge.textContent = `WA: ${waCount} Online`;
      }

      if (usersRes.status === 'fulfilled' && usersRes.value) {
        const u = usersRes.value;
        document.getElementById('metricUsers').textContent = u.total || (u.users ? u.users.length : 0);
      }

      if (campRes.status === 'fulfilled' && campRes.value) {
        const c = campRes.value;
        const list = c.campaigns || [];
        const activeCount = list.filter(item => item.status === 'active' || item.status === 'approved').length;
        const el = document.getElementById('metricCampaigns');
        if (el) el.textContent = activeCount;
      }

      if (withRes.status === 'fulfilled' && withRes.value) {
        const w = withRes.value;
        const list = w.withdrawals || [];
        const pendingCount = list.filter(item => item.status === 'pending').length;
        const el = document.getElementById('metricWithdrawals');
        if (el) el.textContent = pendingCount;
      }

      if (settRes.status === 'fulfilled' && settRes.value && settRes.value.settings) {
        const sett = settRes.value.settings;
        const cEl = document.getElementById('dashAppCode');
        if (cEl) cEl.textContent = sett.app_latest_version_code || '--';
        const nEl = document.getElementById('dashAppName');
        if (nEl) nEl.textContent = sett.app_latest_version_name || '--';
        const fEl = document.getElementById('dashForceUpdate');
        if (fEl) fEl.textContent = sett.app_force_update ? 'ENABLED' : 'DISABLED';

        populateSettingsFields(sett);
      }
    } catch (e) {
      console.warn('Overview fetch error:', e);
    }
  }

  function populateSettingsFields(sett) {
    const setVal = (id, val) => { const el = document.getElementById(id); if (el) el.value = val; };
    const setChk = (id, val) => { const el = document.getElementById(id); if (el) el.checked = !!val; };

    setVal('appVerCode', sett.app_latest_version_code || '');
    setVal('appVerName', sett.app_latest_version_name || '');
    setVal('appDownloadUrl', sett.app_download_url || '');
    setVal('appUpdateMsg', sett.app_update_message || '');
    setChk('appForceUpdate', sett.app_force_update);

    setChk('settWarmupEnabled', sett.warmup_enabled);
    setVal('settWarmup1', sett.warmup_day1_limit || 5);
    setVal('settWarmup2', sett.warmup_day2_limit || 10);
    setVal('settWarmup3', sett.warmup_day3_limit || 15);
    setVal('settMaxHour', sett.safety_max_messages_hour || 20);
    setVal('settMaxDay', sett.safety_max_messages_day || 100);
  }

  async function loadSettingsData() {
    try {
      const res = await apiFetch('/admin/settings/data');
      if (res && res.settings) {
        populateSettingsFields(res.settings);
      }
    } catch (e) {
      console.warn('Settings load error:', e);
    }
  }

  // ==================== USERS ====================
  let userPage = 1;
  async function loadUsers() {
    const q = (document.getElementById('userSearchInput') ? document.getElementById('userSearchInput').value : '').trim();
    const status = document.getElementById('userStatusFilter') ? document.getElementById('userStatusFilter').value : '';
    const tbody = document.getElementById('usersTableBody');
    if (!tbody) return;
    tbody.innerHTML = '<tr><td colspan="7" style="text-align: center; padding: 30px; color: #64748b;">Loading user directory...</td></tr>';

    try {
      const res = await apiFetch(`/admin/users/data?limit=50&page=${userPage}&search=${encodeURIComponent(q)}&status=${encodeURIComponent(status)}`);
      const users = res.users || [];
      const sumEl = document.getElementById('userCountSummary');
      if (sumEl) sumEl.textContent = `Showing ${users.length} users (Page ${userPage})`;
      
      if (users.length === 0) {
        tbody.innerHTML = '<tr><td colspan="7" style="text-align: center; padding: 30px; color: #64748b;">No users found matching query.</td></tr>';
        return;
      }

      tbody.innerHTML = users.map(u => {
        const isSuspended = u.status === 'suspended';
        return `
          <tr>
            <td>
              <a href="javascript:void(0)" onclick="window.admin2ViewUser('${u.id}')" style="color: #0f172a; font-weight: 700; text-decoration: underline;">
                ${u.phone || u.user_id || u.id}
              </a>
              <div style="font-size: 11px; color: #64748b;" class="font-mono">${u.display_name || 'User'}</div>
            </td>
            <td><strong>${u.country_code || '--'}</strong></td>
            <td><strong style="color: #059669; font-size: 14px;">₹${Number(u.balance || 0).toFixed(2)}</strong></td>
            <td>
              <span class="badge ${isSuspended ? 'badge-suspended' : 'badge-active'}">
                ${(u.status || 'ACTIVE').toUpperCase()}
              </span>
            </td>
            <td>
              <button onclick="window.admin2ToggleWithdrawal('${u.id}', ${!u.withdrawals_enabled})" class="btn btn-outline btn-sm">
                ${u.withdrawals_enabled ? 'Allowed ✓' : 'Blocked ✕'}
              </button>
            </td>
            <td style="color: #64748b; font-size: 12px;">${u.created_at ? new Date(u.created_at).toLocaleDateString() : '--'}</td>
            <td class="text-right">
              <button onclick="window.admin2ViewUser('${u.id}')" class="btn btn-outline btn-sm" style="margin-right: 4px;">
                Details
              </button>
              <button onclick="window.admin2ToggleSuspend('${u.id}', '${isSuspended ? 'activate' : 'suspend'}')" class="btn ${isSuspended ? 'btn-emerald' : 'btn-danger-outline'} btn-sm">
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

  window.admin2ToggleSuspend = async function(userId, action) {
    const label = action === 'suspend' ? 'suspend' : 'reactivate';
    if (!confirm(`Are you sure you want to ${label} this user?`)) return;
    try {
      const res = await apiFetch('/admin/users/action', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ user_id: userId, action: action })
      });
      if (res && res.status === 'success') {
        loadUsers();
      } else {
        alert(res.message || 'Action failed');
      }
    } catch (e) {
      alert('Failed to update user status');
    }
  };

  window.admin2ToggleWithdrawal = async function(userId, allow) {
    try {
      const res = await apiFetch('/admin/users/action', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ user_id: userId, action: 'withdrawals', enabled: allow })
      });
      if (res && res.status === 'success') {
        loadUsers();
      } else {
        alert(res.message || 'Action failed');
      }
    } catch (e) {
      alert('Failed to update withdrawal permission');
    }
  };

  window.admin2ViewUser = async function(userId) {
    try {
      const modal = document.getElementById('userDetailModal');
      const content = document.getElementById('userDetailContent');
      if (!modal || !content) return;
      content.innerHTML = '<div style="padding: 20px; text-align: center;">Loading user profile...</div>';
      modal.classList.remove('hidden');

      const res = await apiFetch(`/admin/users/detail?id=${encodeURIComponent(userId)}`);
      if (res && res.status === 'success') {
        const u = res.user;
        content.innerHTML = `
          <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 16px; margin-bottom: 20px;">
            <div class="card" style="padding: 14px; margin: 0;">
              <div style="font-size: 11px; color: #64748b; font-weight: 700;">USER IDENTIFIER</div>
              <div style="font-size: 16px; font-weight: 700; color: #0f172a;">${u.user_id || u.phone || u.id}</div>
              <div style="font-size: 12px; color: #64748b;">${u.display_name}</div>
            </div>
            <div class="card" style="padding: 14px; margin: 0;">
              <div style="font-size: 11px; color: #64748b; font-weight: 700;">BALANCE / EARNINGS</div>
              <div style="font-size: 16px; font-weight: 700; color: #059669;">₹${Number(u.balance || 0).toFixed(2)}</div>
              <div style="font-size: 12px; color: #64748b;">Total Earned: ₹${Number(u.total_earning || 0).toFixed(2)}</div>
            </div>
          </div>
          <div style="display: grid; grid-template-columns: repeat(3, 1fr); gap: 12px; margin-bottom: 20px;">
            <div style="border: 1px solid #e2e8f0; border-radius: 8px; padding: 12px; text-align: center;">
              <div style="font-size: 11px; color: #64748b;">REFERRALS</div>
              <div style="font-size: 18px; font-weight: 800;">${res.referrals ? res.referrals.length : 0}</div>
            </div>
            <div style="border: 1px solid #e2e8f0; border-radius: 8px; padding: 12px; text-align: center;">
              <div style="font-size: 11px; color: #64748b;">SMS TASKS</div>
              <div style="font-size: 18px; font-weight: 800;">${res.sms_tasks ? res.sms_tasks.length : 0}</div>
            </div>
            <div style="border: 1px solid #e2e8f0; border-radius: 8px; padding: 12px; text-align: center;">
              <div style="font-size: 11px; color: #64748b;">WA TASKS</div>
              <div style="font-size: 18px; font-weight: 800;">${res.wa_tasks ? res.wa_tasks.length : 0}</div>
            </div>
          </div>
          <div style="text-align: right;">
            <button onclick="document.getElementById('userDetailModal').classList.add('hidden')" class="btn btn-outline">Close</button>
          </div>
        `;
      } else {
        content.innerHTML = '<div style="color: #b91c1c; padding: 20px;">Could not load user details.</div>';
      }
    } catch (e) {
      alert('Error fetching user profile');
    }
  };

  // ==================== CAMPAIGNS ====================
  async function loadCampaigns() {
    const tbody = document.getElementById('campaignsTableBody');
    if (!tbody) return;
    tbody.innerHTML = '<tr><td colspan="7" style="text-align: center; padding: 30px; color: #64748b;">Loading campaigns...</td></tr>';

    try {
      const res = await apiFetch('/admin/campaigns/data');
      const list = res.campaigns || [];
      if (list.length === 0) {
        tbody.innerHTML = '<tr><td colspan="7" style="text-align: center; padding: 30px; color: #64748b;">No campaigns registered.</td></tr>';
        return;
      }

      tbody.innerHTML = list.map(c => {
        const badgeClass = c.status === 'active' ? 'badge-active' : (c.status === 'paused' ? 'badge-suspended' : 'badge-outline');
        return `
          <tr>
            <td>
              <strong style="color: #0f172a;">${c.name || 'Untitled Campaign'}</strong>
              <div style="font-size: 11px; color: #64748b;" class="font-mono">${c.public_id || c.id}</div>
            </td>
            <td><span class="badge" style="background: #f1f5f9; color: #0f172a;">${(c.channel || 'SMS').toUpperCase()}</span></td>
            <td><strong>${c.country_code || '--'}</strong></td>
            <td>${c.completed_tasks || 0} / ${c.total_tasks || 0}</td>
            <td><strong style="color: #059669;">₹${Number(c.unit_price || 0).toFixed(2)}</strong></td>
            <td><span class="badge ${badgeClass}">${(c.status || 'PENDING').toUpperCase()}</span></td>
            <td class="text-right">
              ${c.status === 'pending_review' ? `
                <button onclick="window.admin2ApproveCampaign('${c.id}')" class="btn btn-emerald btn-sm" style="margin-right: 4px;">Approve</button>
                <button onclick="window.admin2UpdateCampaignStatus('${c.id}', 'rejected')" class="btn btn-danger-outline btn-sm">Reject</button>
              ` : ''}
              ${c.status === 'active' ? `
                <button onclick="window.admin2UpdateCampaignStatus('${c.id}', 'paused')" class="btn btn-outline btn-sm">Pause</button>
              ` : ''}
              ${c.status === 'paused' ? `
                <button onclick="window.admin2UpdateCampaignStatus('${c.id}', 'active')" class="btn btn-emerald btn-sm">Resume</button>
              ` : ''}
              <button onclick="window.admin2ViewCampaign('${c.id}')" class="btn btn-outline btn-sm" style="margin-left: 4px;">View</button>
            </td>
          </tr>
        `;
      }).join('');
    } catch (e) {
      tbody.innerHTML = '<tr><td colspan="7" style="text-align: center; padding: 30px; color: #b91c1c;">Error loading campaigns.</td></tr>';
    }
  }

  window.admin2ApproveCampaign = async function(id) {
    const priceStr = prompt('Enter Unit Price for this campaign (e.g. 0.50):', '0.50');
    if (!priceStr) return;
    const price = parseFloat(priceStr);
    if (isNaN(price) || price <= 0) {
      alert('Invalid unit price');
      return;
    }
    try {
      const res = await apiFetch('/admin/campaigns/action', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id: id, status: 'approved', unit_price: price, admin_notes: 'Approved in Admin 2.0' })
      });
      if (res && res.status === 'success') {
        alert('Campaign approved successfully!');
        loadCampaigns();
      } else {
        alert(res.message || 'Approval failed');
      }
    } catch (e) {
      alert('Failed to approve campaign');
    }
  };

  window.admin2UpdateCampaignStatus = async function(id, status) {
    if (!confirm(`Are you sure you want to mark campaign as ${status}?`)) return;
    try {
      const res = await apiFetch('/admin/campaigns/action', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id: id, status: status })
      });
      if (res && res.status === 'success') {
        loadCampaigns();
      } else {
        alert(res.message || 'Action failed');
      }
    } catch (e) {
      alert('Failed to update campaign status');
    }
  };

  window.admin2ViewCampaign = async function(id) {
    try {
      const res = await apiFetch(`/admin/campaigns/detail?id=${encodeURIComponent(id)}`);
      if (res && res.status === 'success') {
        const c = res.campaign;
        const tasks = res.tasks || [];
        alert(`Campaign: ${c.name}
Channel: ${c.channel}
Total Tasks: ${tasks.length}
Status: ${c.status}`);
      } else {
        alert(res.message || 'Could not load campaign details');
      }
    } catch (e) {
      alert('Error fetching campaign details');
    }
  };

  // ==================== WHATSAPP SESSIONS ====================
  async function loadWhatsApp() {
    const tbody = document.getElementById('waSessionsTableBody');
    if (!tbody) return;
    tbody.innerHTML = '<tr><td colspan="5" style="text-align: center; padding: 30px; color: #64748b;">Loading WhatsApp sessions...</td></tr>';

    try {
      const [devRes, statusRes] = await Promise.all([
        apiFetch('/admin/devices/data'),
        apiFetch('/admin/status')
      ]);

      const devices = devRes.devices || (Array.isArray(devRes) ? devRes : []);
      if (devices.length === 0) {
        tbody.innerHTML = '<tr><td colspan="5" style="text-align: center; padding: 30px; color: #64748b;">No WhatsApp devices connected.</td></tr>';
        return;
      }

      tbody.innerHTML = devices.map(d => {
        const isOnline = d.connected || (d.status === 'connected') || (d.status === 'logged_in');
        return `
          <tr>
            <td>
              <strong style="color: #0f172a; font-size: 14px;">${d.phone || d.user_id || 'Unknown'}</strong>
              <div style="font-size: 11px; color: #64748b;" class="font-mono">${d.user_id}</div>
            </td>
            <td>
              <span class="badge ${isOnline ? 'badge-active' : 'badge-suspended'}">
                ${isOnline ? 'CONNECTED' : 'DISCONNECTED'}
              </span>
            </td>
            <td>${d.push_name || d.name || '--'}</td>
            <td style="color: #64748b; font-size: 12px;">${d.updated_at ? new Date(d.updated_at).toLocaleString() : '--'}</td>
            <td class="text-right">
              <button onclick="window.admin2ReconnectWA('${d.user_id}')" class="btn btn-outline btn-sm">
                Reconnect
              </button>
            </td>
          </tr>
        `;
      }).join('');
    } catch (e) {
      tbody.innerHTML = '<tr><td colspan="5" style="text-align: center; padding: 30px; color: #b91c1c;">Error loading WhatsApp devices.</td></tr>';
    }
  }

  window.admin2ReconnectWA = async function(userId) {
    try {
      const res = await apiFetch(`/admin/reconnect?user_id=${encodeURIComponent(userId)}`, { method: 'POST' });
      alert(res.message || 'Reconnect signal sent');
      loadWhatsApp();
    } catch (e) {
      alert('Reconnect failed');
    }
  };

  // ==================== WITHDRAWALS ====================
  async function loadWithdrawals() {
    const tbody = document.getElementById('withdrawalsTableBody');
    if (!tbody) return;
    tbody.innerHTML = '<tr><td colspan="7" style="text-align: center; padding: 30px; color: #64748b;">Loading withdrawal requests...</td></tr>';

    try {
      const res = await apiFetch('/admin/withdrawals/data');
      const list = res.withdrawals || [];
      if (list.length === 0) {
        tbody.innerHTML = '<tr><td colspan="7" style="text-align: center; padding: 30px; color: #64748b;">No withdrawal requests found.</td></tr>';
        return;
      }

      tbody.innerHTML = list.map(w => {
        const isPending = w.status === 'pending';
        const isPaid = w.status === 'completed' || w.status === 'approved' || w.status === 'paid';
        const badgeClass = isPaid ? 'badge-active' : (isPending ? 'badge-outline' : 'badge-suspended');

        return `
          <tr>
            <td>
              <strong style="color: #0f172a;">${w.name || w.user_id}</strong>
              <div style="font-size: 11px; color: #64748b;" class="font-mono">${w.user_id}</div>
            </td>
            <td><strong>${w.channel_name || '--'}</strong></td>
            <td><strong style="color: #059669; font-size: 14px;">₹${Number(w.net_amount || w.gross_amount || 0).toFixed(2)}</strong></td>
            <td>
              <div style="font-size: 12px; font-family: monospace;">
                ${typeof w.beneficiary === 'object' ? JSON.stringify(w.beneficiary) : (w.beneficiary || '--')}
              </div>
            </td>
            <td><span class="badge ${badgeClass}">${(w.status || 'PENDING').toUpperCase()}</span></td>
            <td style="color: #64748b; font-size: 12px;">${w.created_at ? new Date(w.created_at).toLocaleString() : '--'}</td>
            <td class="text-right">
              ${isPending ? `
                <button onclick="window.admin2ProcessWithdrawal('${w.id}', 'approve')" class="btn btn-emerald btn-sm" style="margin-right: 4px;">Approve</button>
                <button onclick="window.admin2ProcessWithdrawal('${w.id}', 'reject')" class="btn btn-danger-outline btn-sm">Reject</button>
              ` : `
                <span style="font-size: 12px; color: #64748b;">Processed</span>
              `}
            </td>
          </tr>
        `;
      }).join('');
    } catch (e) {
      tbody.innerHTML = '<tr><td colspan="7" style="text-align: center; padding: 30px; color: #b91c1c;">Error loading withdrawals.</td></tr>';
    }
  }

  window.admin2ProcessWithdrawal = async function(id, action) {
    const note = prompt(`Enter optional internal note to ${action} withdrawal:`, '');
    if (note === null) return;
    try {
      const res = await apiFetch('/admin/withdrawals/data', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id: id, action: action, note: note })
      });
      if (res && res.status === 'success') {
        alert(`Withdrawal marked as ${action}!`);
        loadWithdrawals();
      } else {
        alert(res.message || 'Operation failed');
      }
    } catch (e) {
      alert('Failed to process withdrawal');
    }
  };

  // ==================== BROADCASTS / PUSH ====================
  async function loadBroadcasts() {
    const tbody = document.getElementById('fcmHistoryTableBody');
    if (!tbody) return;
    tbody.innerHTML = '<tr><td colspan="6" style="text-align: center; padding: 20px; color: #64748b;">Loading notification history...</td></tr>';

    try {
      const res = await apiFetch('/admin/fcm/history');
      const list = res.notifications || (Array.isArray(res) ? res : []);
      if (list.length === 0) {
        tbody.innerHTML = '<tr><td colspan="6" style="text-align: center; padding: 20px; color: #64748b;">No past notifications found.</td></tr>';
        return;
      }

      tbody.innerHTML = list.map(item => `
        <tr>
          <td><strong style="color: #0f172a;">${item.title || '--'}</strong></td>
          <td style="color: #475569; font-size: 13px;">${item.body || '--'}</td>
          <td><span class="badge badge-outline">${(item.target_type || 'ALL').toUpperCase()}</span></td>
          <td><strong style="color: #059669;">${item.sent_count || 0}</strong> / <span style="color: #dc2626;">${item.failed_count || 0}</span></td>
          <td><span class="badge ${item.status === 'sent' ? 'badge-active' : 'badge-suspended'}">${(item.status || 'SENT').toUpperCase()}</span></td>
          <td style="color: #64748b; font-size: 12px;">${item.created_at ? new Date(item.created_at).toLocaleString() : '--'}</td>
        </tr>
      `).join('');
    } catch (e) {
      tbody.innerHTML = '<tr><td colspan="6" style="text-align: center; padding: 20px; color: #b91c1c;">Could not load notification history.</td></tr>';
    }
  }

  const sendPushBtn = document.getElementById('sendPushBtn');
  if (sendPushBtn) {
    sendPushBtn.addEventListener('click', async () => {
      const title = document.getElementById('pushTitle').value.trim();
      const body = document.getElementById('pushBody').value.trim();
      const imageUrl = document.getElementById('pushImageUrl').value.trim();
      if (!title || !body) {
        alert('Please provide Title and Body for the notification');
        return;
      }
      try {
        const res = await apiFetch('/admin/fcm/broadcast', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ title: title, body: body, image_url: imageUrl, target: 'all' })
        });
        alert(res.message || 'Notification broadcast completed!');
        loadBroadcasts();
      } catch (e) {
        alert('Broadcast failed: ' + (e.message || 'API error'));
      }
    });
  }

  // ==================== APP RELEASE ====================
  const saveAppReleaseBtn = document.getElementById('saveAppReleaseBtn');
  if (saveAppReleaseBtn) {
    saveAppReleaseBtn.addEventListener('click', async () => {
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
        alert('App version settings saved successfully!');
        loadOverview();
      } catch (e) {
        alert('Failed to save version settings');
      }
    });
  }

  // ==================== ANTI-BAN SAFETY SETTINGS ====================
  const saveAntiBanBtn = document.getElementById('saveAntiBanBtn');
  if (saveAntiBanBtn) {
    saveAntiBanBtn.addEventListener('click', async () => {
      const enabled = document.getElementById('settWarmupEnabled').checked;
      const w1 = parseInt(document.getElementById('settWarmup1').value, 10) || 5;
      const w2 = parseInt(document.getElementById('settWarmup2').value, 10) || 10;
      const w3 = parseInt(document.getElementById('settWarmup3').value, 10) || 15;
      const maxHr = parseInt(document.getElementById('settMaxHour').value, 10) || 20;
      const maxDay = parseInt(document.getElementById('settMaxDay').value, 10) || 100;

      try {
        await apiFetch('/admin/settings/data', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            warmup_enabled: enabled,
            warmup_day1_limit: w1,
            warmup_day2_limit: w2,
            warmup_day3_limit: w3,
            safety_max_messages_hour: maxHr,
            safety_max_messages_day: maxDay
          })
        });
        alert('Anti-ban warm-up safety settings saved!');
      } catch (e) {
        alert('Failed to save anti-ban settings');
      }
    });
  }

  // Fast pair button
  const quickPairBtn = document.getElementById('quickPairBtn');
  if (quickPairBtn) {
    quickPairBtn.addEventListener('click', async () => {
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
  }

  // Search & Pagination Listeners
  const userSearch = document.getElementById('userSearchInput');
  if (userSearch) userSearch.addEventListener('input', debounce(loadUsers, 400));
  const userStatus = document.getElementById('userStatusFilter');
  if (userStatus) userStatus.addEventListener('change', loadUsers);
  const prevBtn = document.getElementById('userPrevPage');
  if (prevBtn) prevBtn.addEventListener('click', () => { if (userPage > 1) { userPage--; loadUsers(); } });
  const nextBtn = document.getElementById('userNextPage');
  if (nextBtn) nextBtn.addEventListener('click', () => { userPage++; loadUsers(); });
  const refreshBtn = document.getElementById('refreshAllBtn');
  if (refreshBtn) refreshBtn.addEventListener('click', loadAllData);

  function debounce(func, wait) {
    let timeout;
    return function(...args) {
      clearTimeout(timeout);
      timeout = setTimeout(() => func.apply(this, args), wait);
    };
  }

  function loadAllData() {
    loadOverview();
    const hash = window.location.hash.replace('#', '') || 'overview';
    switchTab(hash);
  }

  if (!currentToken) {
    showLoginModal();
  } else {
    loadAllData();
  }

})();
