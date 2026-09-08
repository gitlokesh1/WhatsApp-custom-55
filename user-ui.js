(() => {
  const nav = [
    ['dashboard','/dashboard','▦','Home'],['tasks','/tasks','✓','Tasks'],['whatsapp','/whatsapp','◉','WhatsApp'],['referrals','/referrals','♢','Referrals'],['profile','/profile','○','Profile'],['support','/support','?','Support']
  ];
  const escapeHTML = value => String(value ?? '').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
  const money = (amount, currency = 'INR') => { try { return new Intl.NumberFormat(undefined,{style:'currency',currency:currency||'INR',maximumFractionDigits:2}).format(Number(amount)||0); } catch (_) { return `${currency||''} ${(Number(amount)||0).toFixed(2)}`; } };
  const date = value => value ? new Date(value).toLocaleString() : '—';
  const api = async (url, options = {}) => {
    const response = await fetch(url, options);
    const data = await response.json().catch(() => ({}));
    if (response.status === 401) { location.href = '/login'; throw new Error('Login required'); }
    if (!response.ok || data.status === 'error') throw new Error(data.message || 'Something went wrong');
    return data;
  };
  const toast = (message, type = '') => {
    let stack = document.querySelector('.u-toast-stack'); if (!stack) { stack=document.createElement('div');stack.className='u-toast-stack';stack.setAttribute('aria-live','polite');document.body.appendChild(stack); }
    const item=document.createElement('div');item.className=`u-toast ${type}`;item.textContent=message;stack.appendChild(item);setTimeout(()=>item.remove(),4200);
  };
  const empty = (icon,title,copy='') => `<div class="u-empty"><div class="u-empty-icon">${icon}</div><strong>${escapeHTML(title)}</strong>${copy?`<div>${escapeHTML(copy)}</div>`:''}</div>`;
  const logout = async () => { try { await fetch('/logout',{method:'POST'}); } finally { location.href='/login'; } };
  function shell(page) {
    const main=document.querySelector('main[data-user-page]'); if(!main||main.closest('.u-shell'))return;
    const links=nav.map(([key,href,icon,label])=>`<a href="${href}" class="${key===page?'active':''}" ${key===page?'aria-current="page"':''}><span class="u-nav-icon">${icon}</span><span>${label}</span></a>`).join('');
    const shell=document.createElement('div');shell.className='u-shell';shell.innerHTML=`<aside class="u-sidebar"><a class="u-brand" href="/dashboard"><span class="u-brand-mark">88</span><span><strong>88Task</strong><small>Earn with every task</small></span></a><nav class="u-nav">${links}</nav><div class="u-sidebar-foot"><p>Complete verified tasks, grow your streak, and build your rewards.</p><button class="u-logout" type="button">↪ &nbsp; Sign out</button></div></aside><section class="u-workspace"><header class="u-topbar"><div class="u-topbar-copy"><strong id="uTopGreeting">88Task workspace</strong><span>Secure WhatsApp task rewards</span></div><div class="u-wallet-pill">● <span id="uTopBalance">—</span></div></header></section><nav class="u-mobile-nav">${nav.slice(0,5).map(([key,href,icon,label])=>`<a href="${href}" class="${key===page?'active':''}"><span>${icon}</span><span>${label}</span></a>`).join('')}</nav>`;
    main.parentNode.insertBefore(shell,main);shell.querySelector('.u-workspace').appendChild(main);shell.querySelector('.u-logout').addEventListener('click',logout);
  }
  async function chooseCountry() {
    const data=await api('/api/public/countries');
    const root=document.createElement('div');root.className='u-modal-root show';root.innerHTML=`<div class="u-modal" role="dialog" aria-modal="true" aria-labelledby="countryTitle"><div class="u-eyebrow">One-time setup</div><h2 id="countryTitle">Choose your earning country</h2><p>Your country sets your currency, message reward, and daily goal. It locks after your first earning.</p><div class="u-field"><label for="countryChoice">Country</label><select class="u-select" id="countryChoice"><option value="">Select country</option>${data.countries.map(c=>`<option value="${escapeHTML(c.code)}">${escapeHTML(c.name)} · ${escapeHTML(c.currency_code)} · ${money(c.reward_per_message,c.currency_code)}/message</option>`).join('')}</select></div><button class="u-btn primary block" id="saveCountry" style="margin-top:16px">Confirm country</button><p class="u-help" id="countryError" style="margin-top:10px"></p></div>`;document.body.appendChild(root);
    return new Promise(resolve=>root.querySelector('#saveCountry').addEventListener('click',async()=>{const code=root.querySelector('#countryChoice').value;if(!code){root.querySelector('#countryError').textContent='Please choose a country.';return}try{await api('/api/user/country',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({country_code:code})});root.remove();resolve()}catch(error){root.querySelector('#countryError').textContent=error.message}}));
  }
  async function profile(requireCountry=true) {
    let data=await api('/api/user/profile'); if(data.requires_country&&requireCountry){await chooseCountry();data=await api('/api/user/profile')}
    const greeting=document.getElementById('uTopGreeting');if(greeting)greeting.textContent=`Hello, ${data.name}`;const balance=document.getElementById('uTopBalance');if(balance)balance.textContent=money(data.balance,data.currency_code);window.userProfile=data;return data;
  }
  function celebrate(reward,currency,gamification){const root=document.createElement('div');root.className='u-modal-root show';root.innerHTML=`<div class="u-modal u-celebrate" role="dialog" aria-modal="true"><div class="u-celebrate-icon">🎉</div><div class="u-eyebrow">Reward unlocked</div><h2>${money(reward,currency)} earned!</h2><p>Your verified message was sent successfully. You are now on a ${gamification?.streak_days||0}-day streak.</p><button class="u-btn primary" type="button">Keep earning</button></div>`;document.body.appendChild(root);root.querySelector('button').onclick=()=>root.remove()}
  document.addEventListener('DOMContentLoaded',()=>{const main=document.querySelector('main[data-user-page]');if(main)shell(main.dataset.userPage)});
  Object.assign(window,{uEscape:escapeHTML,uMoney:money,uDate:date,uApi:api,uToast:toast,uEmpty:empty,uProfile:profile,uCelebrate:celebrate,uLogout:logout});
})();
