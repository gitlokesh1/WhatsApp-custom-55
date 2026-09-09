(() => {
  const selector = '.u-brand-mark,.admin-brand-mark,.company-logo-slot';

  const style = document.createElement('style');
  style.textContent = '.company-logo-image{display:block;width:100%;height:100%;object-fit:contain;border-radius:inherit}.company-logo-slot{width:43px;height:43px;display:inline-grid;place-items:center;flex:0 0 auto;border-radius:13px;background:#087a50;color:#fff;font-weight:900;font-size:16px;overflow:hidden}';
  document.head.appendChild(style);

  function applyLogo(root, logoURL) {
    const nodes = [
      ...(root.matches?.(selector) ? [root] : []),
      ...(root.querySelectorAll?.(selector) || [])
    ];
    nodes.forEach(node => {
      if (!logoURL || node.dataset.companyLogo === logoURL) return;
      const fallback = [...node.childNodes].map(child => child.cloneNode(true));
      const image = document.createElement('img');
      image.src = logoURL;
      image.alt = 'Company logo';
      image.className = 'company-logo-image';
      image.addEventListener('error', () => {
        node.replaceChildren(...fallback);
        delete node.dataset.companyLogo;
      }, { once: true });
      node.replaceChildren(image);
      node.dataset.companyLogo = logoURL;
      node.removeAttribute('aria-hidden');
    });
  }

  async function init() {
    try {
      const response = await fetch('/api/public/branding', { cache: 'no-cache' });
      const data = await response.json();
      if (!response.ok || !data.logo_url) return;
      applyLogo(document, data.logo_url);
      new MutationObserver(records => records.forEach(record => record.addedNodes.forEach(node => {
        if (node.nodeType === Node.ELEMENT_NODE) applyLogo(node, data.logo_url);
      }))).observe(document.body, { childList: true, subtree: true });
    } catch (_) {
      // The built-in 88 placeholder remains when branding is unavailable.
    }
  }

  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', init);
  else init();
})();
