async function api(path, opts = {}) {
  const res = await fetch('api' + path, {
    method: opts.method || 'GET',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'same-origin',
    body: opts.body ? JSON.stringify(opts.body) : undefined,
  });
  let data = null;
  try { data = await res.json(); } catch (e) { /* no body */ }
  if (!res.ok) {
    throw new Error((data && data.error) || `Request failed (${res.status})`);
  }
  return data;
}

async function getMe() {
  try { return await api('/me'); } catch (e) { return null; }
}

function renderNav(activePage, me) {
  const nav = document.getElementById('nav');
  if (!nav) return;
  const links = [
    { href: 'index.html', label: 'My Picks', page: 'picks' },
    { href: 'all-picks.html', label: 'All Picks', page: 'all-picks' },
    { href: 'leaderboard.html', label: 'Leaderboard', page: 'leaderboard' },
  ];
  if (me && me.isAdmin) links.push({ href: 'admin.html', label: 'Admin', page: 'admin' });

  let html = `<div class="brand">\u26be Pick'em</div><div class="nav-links">`;
  for (const l of links) {
    html += `<a href="${l.href}" class="${l.page === activePage ? 'active' : ''}">${l.label}</a>`;
  }
  if (me) {
    html += `<span class="muted" style="color:#cdd6e3;margin-left:8px;">${me.displayName}</span>`;
    html += `<button class="linklike" id="logoutBtn">Log out</button>`;
  } else {
    html += `<a href="login.html">Log in</a>`;
  }
  html += `</div>`;
  nav.innerHTML = html;

  const logoutBtn = document.getElementById('logoutBtn');
  if (logoutBtn) {
    logoutBtn.addEventListener('click', async () => {
      await api('/logout', { method: 'POST' });
      window.location.href = 'login.html';
    });
  }
}

function fmtDate(iso) {
  if (!iso) return 'TBD';
  const d = new Date(iso);
  return d.toLocaleString(undefined, { month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit' });
}

// Mirrors the backend's mvpMatches in scoring.go: case/whitespace
// insensitive, and matches a last-name-only pick ("Ohtani") against a
// full name ("Shohei Ohtani") by requiring every word in the shorter
// pick to appear somewhere in the longer one.
function mvpMatches(a, b) {
  if (!a || !b) return false;
  const aWords = a.trim().toLowerCase().split(/\s+/).filter(Boolean);
  const bWords = b.trim().toLowerCase().split(/\s+/).filter(Boolean);
  if (aWords.length === 0 || bWords.length === 0) return false;
  const [shorter, longer] = aWords.length <= bWords.length ? [aWords, bWords] : [bWords, aWords];
  return shorter.every(w => longer.includes(w));
}