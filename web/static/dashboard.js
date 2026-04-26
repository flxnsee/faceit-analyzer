const params   = new URLSearchParams(window.location.search);
const nickname = params.get('player') || '';

let allMatches = [];
let kdChart    = null;

async function init() {
    if (!nickname) {
        showError('No player specified. <a href="/">Go back</a>.');
        return;
    }

    try {
        const [player, matches] = await Promise.all([
            fetchJSON(`/api/player/${encodeURIComponent(nickname)}`),
            fetchJSON(`/api/matches/${encodeURIComponent(nickname)}`),
        ]);

        allMatches = Array.isArray(matches) ? matches : [];

        renderHeader(player);
        renderKDChart(30);
        setupToggles();

        show('content');
        hide('loading');
    } catch (err) {
        showError(err.message);
    }
}

function renderHeader(player) {
    const flag    = countryFlag(player.country);
    const avatar  = player.avatar_url
        ? `<img class="player-avatar" src="${player.avatar_url}" alt="" />`
        : `<div class="player-avatar-placeholder"></div>`;

    document.getElementById('player-header').innerHTML = `
    ${avatar}
    <div class="player-info">
      <div class="player-name">
        ${esc(player.nickname)}
        <span class="level-badge">Level ${player.level}</span>
      </div>
      <div class="player-meta">${flag} ${player.country.toUpperCase()}</div>
      <div class="elo-value">${player.current_elo} ELO</div>
    </div>
  `;

    document.title = `${player.nickname} — Faceit Analyzer`;
}

function renderKDChart(days) {
    const cutoff   = Date.now() / 1000 - days * 86400;
    const filtered = allMatches
        .filter(m => m.played_at >= cutoff)
        .slice()
        .sort((a, b) => a.played_at - b.played_at);

    const labels  = filtered.map(m => formatDate(m.played_at));
    const kdData  = filtered.map(m => m.deaths > 0
        ? parseFloat((m.kills / m.deaths).toFixed(2))
        : m.kills);

    const rolling = rollingAvg(kdData, 5);

    const canvas = document.getElementById('kd-chart');
    const ctx    = canvas.getContext('2d');

    if (kdChart) kdChart.destroy();

    const grad = ctx.createLinearGradient(0, 0, 0, canvas.offsetHeight || 300);
    grad.addColorStop(0, 'rgba(255,85,0,0.35)');
    grad.addColorStop(1, 'rgba(255,85,0,0)');

    kdChart = new Chart(ctx, {
        type: 'line',
        data: {
            labels,
            datasets: [
                {
                    label: 'K/D',
                    data: kdData,
                    borderColor: 'rgba(255,85,0,0.4)',
                    backgroundColor: 'transparent',
                    borderWidth: 1,
                    pointRadius: 2,
                    pointHoverRadius: 4,
                    tension: 0.3,
                },
                {
                    label: '5-match avg',
                    data: rolling,
                    borderColor: '#ff5500',
                    backgroundColor: grad,
                    borderWidth: 2,
                    pointRadius: 0,
                    pointHoverRadius: 4,
                    tension: 0.4,
                    fill: true,
                },
            ],
        },
        options: {
            responsive: true,
            interaction: { mode: 'index', intersect: false },
            plugins: {
                legend: {
                    labels: { color: '#8888a8', font: { size: 12 } },
                },
                tooltip: {
                    backgroundColor: '#1a1a24',
                    borderColor: '#2a2a38',
                    borderWidth: 1,
                    titleColor: '#e8e8f0',
                    bodyColor: '#8888a8',
                },
            },
            scales: {
                x: {
                    ticks: {
                        color: '#8888a8',
                        maxTicksLimit: 10,
                        font: { size: 11 },
                    },
                    grid: { color: 'rgba(255,255,255,0.04)' },
                },
                y: {
                    ticks: { color: '#8888a8', font: { size: 11 } },
                    grid: { color: 'rgba(255,255,255,0.04)' },
                    min: 0,
                },
            },
        },
    });
}

function setupToggles() {
    document.querySelectorAll('.toggle-group button').forEach(btn => {
        btn.addEventListener('click', () => {
            document.querySelectorAll('.toggle-group button').forEach(b => b.classList.remove('active'));
            btn.classList.add('active');
            renderKDChart(parseInt(btn.dataset.days));
        });
    });
}

async function fetchJSON(url) {
    const res = await fetch(url);
    if (!res.ok) throw new Error(`${res.status} — player not found or API error`);
    return res.json();
}

function rollingAvg(data, window) {
    return data.map((_, i) => {
        const slice = data.slice(Math.max(0, i - window + 1), i + 1);
        const avg   = slice.reduce((s, v) => s + v, 0) / slice.length;
        return parseFloat(avg.toFixed(2));
    });
}

function formatDate(unix) {
    return new Date(unix * 1000).toLocaleDateString('en-GB', {
        day: '2-digit', month: 'short',
    });
}

function countryFlag(code) {
    if (!code || code.length !== 2) return '';
    return [...code.toUpperCase()]
        .map(c => String.fromCodePoint(0x1F1E6 - 65 + c.charCodeAt(0)))
        .join('');
}

function esc(str) {
    return str.replace(/[&<>"']/g, c =>
        ({ '&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;' }[c]));
}

function show(id) { document.getElementById(id).classList.remove('hidden'); }
function hide(id) { document.getElementById(id).classList.add('hidden'); }

function showError(msg) {
    hide('loading');
    const el = document.getElementById('error-card');
    el.innerHTML = msg;
    el.classList.remove('hidden');
}

init();