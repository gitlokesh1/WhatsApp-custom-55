(() => {
  window.clientEscape = value => String(value ?? '').replace(/[&<>"']/g, char => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[char]));
  window.clientDate = value => value ? new Date(value).toLocaleString() : '—';
  window.clientApi = async (url, options = {}) => {
    const response = await fetch(url, options);
    const raw = await response.text();
    if (response.status === 401 && location.pathname !== '/client/login') { location.href = '/client/login'; throw new Error('Client login required'); }
    let data = {};
    try { data = raw ? JSON.parse(raw) : {}; } catch (_) { throw new Error(`Request failed (${response.status})`); }
    if (!response.ok || data.status === 'error') throw new Error(data.message || `Request failed (${response.status})`);
    return data;
  };
  window.clientLogout = async () => { try { await fetch('/api/client/logout', {method:'POST'}); } finally { location.href='/client/login'; } };
})();
